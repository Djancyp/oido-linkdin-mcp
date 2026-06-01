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

func NewLinkedInClient() *LinkedInClient {
	baseURL := os.Getenv("LINKEDIN_BASE_URL")
	if baseURL == "" {
		baseURL = "https://www.linkedin.com"
	}

	cookie := os.Getenv("LINKEDIN_COOKIE")
	if cookie == "" {
		log.Println("Warning: LINKEDIN_COOKIE not set — tools will error until configured.")
	}

	csrfToken := strings.Trim(extractCookieValue(cookie, "JSESSIONID"), `"`)

	return &LinkedInClient{
		baseURL:   baseURL,
		cookie:    cookie,
		csrfToken: csrfToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			// Never follow redirects — a redirect means the session is rejected.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *LinkedInClient) Close() {}

func (c *LinkedInClient) checkConfigured() error {
	if c.cookie == "" {
		return fmt.Errorf("LINKEDIN_COOKIE not set — paste full cookie string from browser devtools (linkedin.com, not developer.linkedin.com)")
	}
	if c.csrfToken == "" {
		return fmt.Errorf("JSESSIONID missing from LINKEDIN_COOKIE — make sure you copy the full cookie header")
	}
	return nil
}

func (c *LinkedInClient) newRequest(method, path string, body []byte) (*http.Request, error) {
	u := c.baseURL + path
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", c.cookie)
	req.Header.Set("Csrf-Token", c.csrfToken)
	req.Header.Set("X-RestLi-Protocol-Version", "2.0.0")
	req.Header.Set("X-Li-Lang", "en_US")
	req.Header.Set("X-Li-Track", `{"clientVersion":"1.13.14698","mpVersion":"1.13.14698","osName":"web","timezoneOffset":0,"timezone":"UTC","deviceFormFactor":"DESKTOP","mpName":"voyager-web"}`)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Accept", "application/vnd.linkedin.normalized+json+2.1")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Linux"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Referer", c.baseURL+"/feed/")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req, nil
}

func (c *LinkedInClient) do(req *http.Request) ([]byte, error) {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusMovedPermanently {
		loc := resp.Header.Get("Location")
		return nil, fmt.Errorf("session expired or invalid cookie — LinkedIn redirected to %s. Re-copy the Cookie header from linkedin.com devtools", loc)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 300))
	}
	return data, nil
}

func (c *LinkedInClient) get(path string) ([]byte, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	req, err := c.newRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *LinkedInClient) post(path string, payload interface{}) ([]byte, error) {
	if err := c.checkConfigured(); err != nil {
		return nil, err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := c.newRequest("POST", path, body)
	if err != nil {
		return nil, err
	}
	return c.do(req)
}

func (c *LinkedInClient) doDelete(path string) error {
	if err := c.checkConfigured(); err != nil {
		return err
	}
	req, err := c.newRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	_, err = c.do(req)
	return err
}

// --- Cookie helpers ---

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

// --- Types ---

type Company struct {
	ID          string `json:"entityUrn"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Website     string `json:"website,omitempty"`
}

type Profile struct {
	EntityUrn string `json:"entityUrn"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Headline  string `json:"headline,omitempty"`
	PublicID  string `json:"publicIdentifier,omitempty"`
}

// --- Company ---

func (c *LinkedInClient) GetCompany(companyID string) (json.RawMessage, error) {
	path := fmt.Sprintf("/voyager/api/organization/companies?decorationId=com.linkedin.voyager.deco.organization.web.WebFullCompanyMain-12&q=universalName&universalName=%s", url.QueryEscape(companyID))
	data, err := c.get(path)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) PostAsCompany(companyURN, text string, mediaURNs []string) (json.RawMessage, error) {
	content := map[string]interface{}{
		"author":         companyURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]interface{}{
			"com.linkedin.ugc.ShareContent": map[string]interface{}{
				"shareCommentary":    map[string]string{"text": text},
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
			media = append(media, map[string]interface{}{"status": "READY", "media": urn})
		}
		share := content["specificContent"].(map[string]interface{})["com.linkedin.ugc.ShareContent"].(map[string]interface{})
		share["shareMediaCategory"] = "IMAGE"
		share["media"] = media
	}
	data, err := c.post("/voyager/api/ugcPosts", content)
	return json.RawMessage(data), err
}

// --- Messaging ---

func (c *LinkedInClient) ListConversations(limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/messaging/conversations?keyVersion=LEGACY_INBOX&limit=%d", limit)
	data, err := c.get(path)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) GetConversation(conversationID string, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/messaging/conversations/%s/events?keyVersion=LEGACY_INBOX&limit=%d", url.PathEscape(conversationID), limit)
	data, err := c.get(path)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) SendMessage(conversationID, text string) (json.RawMessage, error) {
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
	data, err := c.post(path, payload)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) NewConversation(recipientURN, text string) (json.RawMessage, error) {
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
	data, err := c.post("/voyager/api/messaging/conversations", payload)
	return json.RawMessage(data), err
}

// --- Connections ---

func (c *LinkedInClient) ListConnections(limit, start int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/relationships/dash/connections?decorationId=com.linkedin.voyager.dash.deco.web.mynetwork.ConnectionListWithProfile-5&count=%d&start=%d&q=viewer&sortType=RECENTLY_CONNECTED", limit, start)
	data, err := c.get(path)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) SendConnectionRequest(profileURN, message string) (json.RawMessage, error) {
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
				"message":       message,
				"customMessage": message != "",
			},
		},
	}
	data, err := c.post("/voyager/api/growth/normInvitations", payload)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) ListPendingRequests(limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/relationships/sentInvitationViewsV2?invitationType=CONNECTION&limit=%d&q=relationship&relationshipType=SENT_RECEIVED&start=0", limit)
	data, err := c.get(path)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) WithdrawRequest(invitationID, sharedSecret string) error {
	payload := map[string]interface{}{
		"invitationID":        invitationID,
		"sharedSecret":        sharedSecret,
		"isGenericInvitation": false,
	}
	_, err := c.post("/voyager/api/relationships/invitations/"+invitationID+"?action=withdraw", payload)
	return err
}

func (c *LinkedInClient) AcceptRequest(invitationID, sharedSecret string) error {
	payload := map[string]interface{}{
		"invitationID": invitationID,
		"sharedSecret": sharedSecret,
	}
	_, err := c.post("/voyager/api/relationships/invitations/"+invitationID+"?action=accept", payload)
	return err
}

// --- Content ---

func (c *LinkedInClient) GetFeed(limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/feed/updatesV2?includeLongTermActivities=true&limit=%d&q=chronological&start=0&updatesTab=ALL_UPDATES", limit)
	data, err := c.get(path)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) CreatePost(authorURN, text string) (json.RawMessage, error) {
	payload := map[string]interface{}{
		"author":         authorURN,
		"lifecycleState": "PUBLISHED",
		"specificContent": map[string]interface{}{
			"com.linkedin.ugc.ShareContent": map[string]interface{}{
				"shareCommentary":    map[string]string{"text": text},
				"shareMediaCategory": "NONE",
			},
		},
		"visibility": map[string]string{
			"com.linkedin.ugc.MemberNetworkVisibility": "PUBLIC",
		},
	}
	data, err := c.post("/voyager/api/ugcPosts", payload)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) LikePost(activityURN string) error {
	payload := map[string]interface{}{"reactionType": "LIKE"}
	_, err := c.post(fmt.Sprintf("/voyager/api/reactions?action=like&urnId=%s", url.QueryEscape(activityURN)), payload)
	return err
}

func (c *LinkedInClient) CommentOnPost(activityURN, text string) (json.RawMessage, error) {
	payload := map[string]interface{}{
		"actor": "",
		"message": map[string]interface{}{
			"text":       text,
			"attributes": []interface{}{},
		},
	}
	path := fmt.Sprintf("/voyager/api/feed/comments?updateKey=%s", url.QueryEscape(activityURN))
	data, err := c.post(path, payload)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) SearchPosts(keywords string, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/search/hits?decorationId=com.linkedin.voyager.deco.search.SearchClusterCollection-6&count=%d&filters=List()&origin=GLOBAL_SEARCH_HEADER&q=all&query=%s&queryContext=List(spellCorrectionEnabled->true)&start=0", limit, url.QueryEscape(keywords))
	data, err := c.get(path)
	return json.RawMessage(data), err
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
