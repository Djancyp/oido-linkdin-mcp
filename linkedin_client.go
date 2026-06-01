package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type LinkedInClient struct {
	baseURL string
	// cookie and csrfToken are read from env on first use (lazy init)
	ready bool
}

// browserState holds the live browser session — initialized lazily on first tool call.
type browserState struct {
	csrfToken string
	browser   *rod.Browser
	page      *rod.Page
}

var bs *browserState

func NewLinkedInClient() *LinkedInClient {
	baseURL := os.Getenv("LINKEDIN_BASE_URL")
	if baseURL == "" {
		baseURL = "https://www.linkedin.com"
	}
	return &LinkedInClient{baseURL: baseURL}
}

// ensureBrowser launches Chrome and injects cookies on first call.
func ensureBrowser(baseURL string) (*browserState, error) {
	if bs != nil {
		return bs, nil
	}

	cookie := os.Getenv("LINKEDIN_COOKIE")
	if cookie == "" {
		return nil, fmt.Errorf("LINKEDIN_COOKIE not set — paste full cookie string from browser devtools")
	}

	csrfToken := strings.Trim(extractCookieValue(cookie, "JSESSIONID"), `"`)
	if csrfToken == "" {
		return nil, fmt.Errorf("JSESSIONID missing from LINKEDIN_COOKIE — paste the full cookie string including JSESSIONID")
	}

	l := launcher.New().
		Headless(true).
		NoSandbox(true).
		Set("disable-dev-shm-usage", "").
		Set("disable-gpu", "")
	// CHROME_BIN env lets Docker/CI pin the exact binary path
	if bin := os.Getenv("CHROME_BIN"); bin != "" {
		l = l.Bin(bin)
	} else if path, found := launcher.LookPath(); found {
		l = l.Bin(path)
	}
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u).MustConnect()

	page, err := browser.Page(proto.TargetCreateTarget{URL: "about:blank"})
	if err != nil {
		return nil, fmt.Errorf("failed to open page: %w", err)
	}

	if err := page.SetCookies(parseCookies(cookie, ".linkedin.com")); err != nil {
		return nil, fmt.Errorf("failed to set cookies: %w", err)
	}

	if err := page.Navigate(baseURL); err != nil {
		log.Printf("Warning: initial LinkedIn navigation failed: %v", err)
	}
	_ = page.WaitLoad()

	bs = &browserState{csrfToken: csrfToken, browser: browser, page: page}
	log.Println("Browser session ready.")
	return bs, nil
}

func (c *LinkedInClient) Close() {
	if bs != nil && bs.browser != nil {
		_ = bs.browser.Close()
		bs = nil
	}
}

// eval runs an async JS function in the browser context and returns the text result.
func eval(b *browserState, script string) (string, error) {
	res, err := b.page.Eval(script)
	if err != nil {
		return "", err
	}
	s := res.Value.Str()
	if s == "" {
		raw := res.Value.Raw()
		b2, err2 := json.Marshal(raw)
		if err2 == nil {
			s = string(b2)
		}
	}
	return s, nil
}

func (c *LinkedInClient) get(path string) ([]byte, error) {
	b, err := ensureBrowser(c.baseURL)
	if err != nil {
		return nil, err
	}
	u := c.baseURL + path
	script := fmt.Sprintf(`async () => {
		const r = await fetch(%q, {
			credentials: 'include',
			headers: {
				'Accept': 'application/vnd.linkedin.normalized+json+2.1',
				'X-Requested-With': 'XMLHttpRequest',
				'X-RestLi-Protocol-Version': '2.0.0',
				'Csrf-Token': %q,
				'X-Li-Lang': 'en_US',
			}
		});
		if (!r.ok) {
			const body = await r.text().catch(() => '');
			throw new Error('HTTP ' + r.status + ': ' + body.slice(0, 300));
		}
		return r.text();
	}`, u, b.csrfToken)
	text, err := eval(b, script)
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

func (c *LinkedInClient) post(path string, payload interface{}) ([]byte, error) {
	b, err := ensureBrowser(c.baseURL)
	if err != nil {
		return nil, err
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	u := c.baseURL + path
	script := fmt.Sprintf(`async () => {
		const r = await fetch(%q, {
			method: 'POST',
			credentials: 'include',
			headers: {
				'Accept': 'application/vnd.linkedin.normalized+json+2.1',
				'Content-Type': 'application/json',
				'X-Requested-With': 'XMLHttpRequest',
				'X-RestLi-Protocol-Version': '2.0.0',
				'Csrf-Token': %q,
				'X-Li-Lang': 'en_US',
			},
			body: JSON.stringify(%s)
		});
		if (!r.ok) {
			const body = await r.text().catch(() => '');
			throw new Error('HTTP ' + r.status + ': ' + body.slice(0, 300));
		}
		return r.text();
	}`, u, b.csrfToken, string(bodyBytes))
	text, err := eval(b, script)
	if err != nil {
		return nil, err
	}
	return []byte(text), nil
}

func (c *LinkedInClient) doDelete(path string) error {
	b, err := ensureBrowser(c.baseURL)
	if err != nil {
		return err
	}
	u := c.baseURL + path
	script := fmt.Sprintf(`async () => {
		const r = await fetch(%q, {
			method: 'DELETE',
			credentials: 'include',
			headers: {
				'Accept': 'application/vnd.linkedin.normalized+json+2.1',
				'X-Requested-With': 'XMLHttpRequest',
				'X-RestLi-Protocol-Version': '2.0.0',
				'Csrf-Token': %q,
				'X-Li-Lang': 'en_US',
			}
		});
		if (!r.ok) {
			const body = await r.text().catch(() => '');
			throw new Error('HTTP ' + r.status + ': ' + body.slice(0, 300));
		}
		return r.text();
	}`, u, b.csrfToken)
	_, err = eval(b, script)
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

func parseCookies(cookieStr, domain string) []*proto.NetworkCookieParam {
	var out []*proto.NetworkCookieParam
	for _, part := range strings.Split(cookieStr, ";") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(part[:idx])
		value := strings.TrimSpace(part[idx+1:])
		if name == "" {
			continue
		}
		out = append(out, &proto.NetworkCookieParam{
			Name:   name,
			Value:  value,
			Domain: domain,
			Path:   "/",
		})
	}
	return out
}

// --- Types (kept for reference in handlers) ---

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
	path := fmt.Sprintf("/voyager/api/organization/companies?decorationId=com.linkedin.voyager.deco.organization.web.WebFullCompanyMain-12&q=universalName&universalName=%s", companyID)
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
	path := fmt.Sprintf("/voyager/api/messaging/conversations/%s/events?keyVersion=LEGACY_INBOX&limit=%d", conversationID, limit)
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
	path := fmt.Sprintf("/voyager/api/messaging/conversations/%s/events", conversationID)
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
	_, err := c.post(fmt.Sprintf("/voyager/api/reactions?action=like&urnId=%s", activityURN), payload)
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
	path := fmt.Sprintf("/voyager/api/feed/comments?updateKey=%s", activityURN)
	data, err := c.post(path, payload)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) SearchPosts(keywords string, limit int) (json.RawMessage, error) {
	if limit <= 0 {
		limit = 20
	}
	path := fmt.Sprintf("/voyager/api/search/hits?decorationId=com.linkedin.voyager.deco.search.SearchClusterCollection-6&count=%d&filters=List()&origin=GLOBAL_SEARCH_HEADER&q=all&query=%s&queryContext=List(spellCorrectionEnabled->true)&start=0", limit, keywords)
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
