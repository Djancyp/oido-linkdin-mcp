package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type LinkedInClient struct {
	baseURL    string
	cookie     string
	csrfToken  string
	httpClient *http.Client
}

func NewLinkedInClient() (*LinkedInClient, error) {
	baseURL := os.Getenv("LINKEDIN_BASE_URL")
	if baseURL == "" {
		baseURL = "https://www.linkedin.com"
	}

	cookie := os.Getenv("LINKEDIN_COOKIE")
	if cookie == "" {
		log.Println("Warning: LINKEDIN_COOKIE not set. Tools will return errors until configured.")
	}

	csrfToken := extractCookieValue(cookie, "JSESSIONID")
	// JSESSIONID is quoted in the cookie string: ajax:XXXX → strip quotes
	csrfToken = strings.Trim(csrfToken, `"`)

	return &LinkedInClient{
		baseURL:   baseURL,
		cookie:    cookie,
		csrfToken: csrfToken,
		httpClient: &http.Client{
			// Never follow redirects — a redirect means LinkedIn rejected auth (→ login page).
			// Surfacing the 302 as an error is cleaner than an infinite redirect loop.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func extractCookieValue(cookieStr, name string) string {
	for _, part := range strings.Split(cookieStr, ";") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		k := strings.TrimSpace(part[:idx])
		v := strings.TrimSpace(part[idx+1:])
		if k == name {
			return v
		}
	}
	return ""
}

func (c *LinkedInClient) checkConfigured() error {
	if c.cookie == "" {
		return fmt.Errorf("LinkedIn not configured: set LINKEDIN_COOKIE")
	}
	if c.csrfToken == "" {
		return fmt.Errorf("LinkedIn JSESSIONID missing from LINKEDIN_COOKIE — paste full cookie string from browser devtools")
	}
	return nil
}

func (c *LinkedInClient) newRequest(method, path string, body []byte) (*http.Request, error) {
	u := c.baseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Csrf-Token", c.csrfToken)
	req.Header.Set("X-RestLi-Protocol-Version", "2.0.0")
	req.Header.Set("X-Li-Lang", "en_US")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/vnd.linkedin.normalized+json+2.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *LinkedInClient) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		loc := resp.Header.Get("Location")
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d redirect to %s — session expired or LINKEDIN_COOKIE invalid; refresh cookie from browser devtools", resp.StatusCode, loc)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode >= 400 {
		return nil, resp.StatusCode, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}
	return data, resp.StatusCode, nil
}

func (c *LinkedInClient) get(path string) ([]byte, error) {
	req, err := c.newRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	data, _, err := c.do(req)
	return data, err
}

func (c *LinkedInClient) post(path string, payload interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	data, _, err := c.do(req)
	return data, err
}

func (c *LinkedInClient) delete(path string) error {
	req, err := c.newRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	_, _, err = c.do(req)
	return err
}

// --- Types ---

type Company struct {
	ID          string `json:"entityUrn"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Website     string `json:"website,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Staff       int    `json:"staffCount,omitempty"`
}

type Conversation struct {
	EntityUrn      string    `json:"entityUrn"`
	LastActivity   int64     `json:"lastActivityAt"`
	UnreadCount    int       `json:"unreadCount"`
	Participants   []Profile `json:"participants,omitempty"`
}

type Message struct {
	EntityUrn string `json:"entityUrn"`
	Body      string `json:"body"`
	SentAt    int64  `json:"deliveredAt"`
	Sender    string `json:"senderUrn"`
}

type Profile struct {
	EntityUrn  string `json:"entityUrn"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Headline   string `json:"headline,omitempty"`
	PublicID   string `json:"publicIdentifier,omitempty"`
}

type Connection struct {
	EntityUrn string  `json:"entityUrn"`
	Profile   Profile `json:"memberProfile"`
	ConnectedAt int64 `json:"createdAt"`
}

type Post struct {
	EntityUrn   string `json:"entityUrn"`
	Commentary  string `json:"commentary,omitempty"`
	PublishedAt int64  `json:"publishedAt,omitempty"`
	Author      string `json:"author,omitempty"`
}

// --- Company ---

func (c *LinkedInClient) GetCompany(companyID string) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/voyager/api/organization/companies?decorationId=com.linkedin.voyager.deco.organization.web.WebFullCompanyMain-12&q=universalName&universalName=%s",
		url.QueryEscape(companyID))
	return c.get(path)
}

func (c *LinkedInClient) PostAsCompany(companyURN, text string, mediaURNs []string) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	content := map[string]interface{}{
		"author":         companyURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]interface{}{
			"com.linkedin.ugc.ShareContent": map[string]interface{}{
				"shareCommentary": map[string]string{
					"text": text,
				},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]string{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}
	if len(mediaURNs) > 0 {
		media := make([]map[string]interface{}, 0, len(mediaURNs))
		for _, urn := range mediaURNs {
			media = append(media, map[string]interface{}{
				"status":    "READY",
				"media":     urn,
			})
		}
		content["specificContent"].(map[string]interface{})["com.linkedin.ugc.ShareContent"].(map[string]interface{})["shareMediaCategory"] = "IMAGE"
		content["specificContent"].(map[string]interface{})["com.linkedin.ugc.ShareContent"].(map[string]interface{})["media"] = media
	}
	return c.post("/voyager/api/ugcPosts", content)
}

// --- Messaging ---

func (c *LinkedInClient) ListConversations(limit int) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/messaging/conversations?keyVersion=LEGACY_INBOX&limit=%d", limit)
	return c.get(path)
}

func (c *LinkedInClient) GetConversation(conversationID string, limit int) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/messaging/conversations/%s/events?keyVersion=LEGACY_INBOX&limit=%d",
		url.PathEscape(conversationID), limit)
	return c.get(path)
}

func (c *LinkedInClient) SendMessage(conversationID, text string) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"eventCreate": map[string]interface{}{
			"value": map[string]interface{}{
				"com.linkedin.voyager.messaging.create.MessageCreate": map[string]interface{}{
					"attributedBody": map[string]interface{}{
						"text":       text,
						"attributes": []interface{}{},
					},
					"attachments": []interface{}{},
				},
			},
		},
	}
	path := fmt.Sprintf("/voyager/api/messaging/conversations/%s/events", url.PathEscape(conversationID))
	return c.post(path, payload)
}

func (c *LinkedInClient) NewConversation(recipientURN, text string) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"keyVersion": "LEGACY_INBOX",
		"conversationCreate": map[string]interface{}{
			"recipients": []string{recipientURN},
			"eventCreate": map[string]interface{}{
				"value": map[string]interface{}{
					"com.linkedin.voyager.messaging.create.MessageCreate": map[string]interface{}{
						"attributedBody": map[string]interface{}{
							"text":       text,
							"attributes": []interface{}{},
						},
						"attachments": []interface{}{},
					},
				},
			},
		},
	}
	return c.post("/voyager/api/messaging/conversations", payload)
}

// --- Connections ---

func (c *LinkedInClient) ListConnections(limit int, start int) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/relationships/dash/connections?decorationId=com.linkedin.voyager.dash.deco.web.mynetwork.ConnectionListWithProfile-5&count=%d&start=%d&q=viewer&sortType=RECENTLY_CONNECTED",
		limit, start)
	return c.get(path)
}

func (c *LinkedInClient) SendConnectionRequest(profileURN, message string) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"trackingID": generateTrackingID(),
		"invitations": []map[string]interface{}{
			{
				"trackingID": generateTrackingID(),
				"invitee": map[string]interface{}{
					"com.linkedin.voyager.growth.invitation.InviteeProfile": map[string]string{
						"profileID": profileURN,
					},
				},
				"message":         message,
				"customMessage":   message != "",
			},
		},
	}
	return c.post("/voyager/api/growth/normInvitations", payload)
}

func (c *LinkedInClient) ListPendingRequests(limit int) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/relationships/sentInvitationViewsV2?invitationType=CONNECTION&limit=%d&q=relationship&relationshipType=SENT_RECEIVED&start=0", limit)
	return c.get(path)
}

func (c *LinkedInClient) WithdrawRequest(invitationID, sharedSecret string) error {
	if err := c.checkConfigured(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"invitationID":   invitationID,
		"sharedSecret":   sharedSecret,
		"isGenericInvitation": false,
	}
	_, err := c.post("/voyager/api/relationships/invitations/"+invitationID+"?action=withdraw", payload)
	return err
}

func (c *LinkedInClient) AcceptRequest(invitationID, sharedSecret string) error {
	if err := c.checkConfigured(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"invitationID": invitationID,
		"sharedSecret": sharedSecret,
	}
	_, err := c.post("/voyager/api/relationships/invitations/"+invitationID+"?action=accept", payload)
	return err
}

// --- Content ---

func (c *LinkedInClient) GetFeed(limit int) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/feed/updatesV2?includeLongTermActivities=true&limit=%d&q=chronological&start=0&updatesTab=ALL_UPDATES", limit)
	return c.get(path)
}

func (c *LinkedInClient) CreatePost(authorURN, text string) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"author":         authorURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]interface{}{
			"com.linkedin.ugc.ShareContent": map[string]interface{}{
				"shareCommentary": map[string]string{
					"text": text,
				},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]string{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}
	return c.post("/voyager/api/ugcPosts", payload)
}

func (c *LinkedInClient) LikePost(activityURN string) error {
	if err := c.checkConfigured(); err != nil {
		return err
	}
	payload := map[string]interface{}{
		"reactionType": "LIKE",
	}
	_, err := c.post(fmt.Sprintf("/voyager/api/reactions?action=like&urnId=%s", url.QueryEscape(activityURN)), payload)
	return err
}

func (c *LinkedInClient) CommentOnPost(activityURN, text string) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	payload := map[string]interface{}{
		"actor": "",
		"message": map[string]interface{}{
			"text":       text,
			"attributes": []interface{}{},
		},
	}
	path := fmt.Sprintf("/voyager/api/feed/comments?updateKey=%s", url.QueryEscape(activityURN))
	return c.post(path, payload)
}

func (c *LinkedInClient) SearchPosts(keywords string, limit int) (json.RawMessage, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/search/hits?decorationId=com.linkedin.voyager.deco.search.SearchClusterCollection-6&count=%d&filters=List()&origin=GLOBAL_SEARCH_HEADER&q=all&query=%s&queryContext=List(spellCorrectionEnabled->true)&start=0",
		limit, url.QueryEscape(keywords))
	return c.get(path)
}

// --- Helpers ---

func generateTrackingID() string {
	return fmt.Sprintf("%d", time.Now().UnixMilli())
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
