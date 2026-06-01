package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	authURL  = "https://www.linkedin.com/oauth/v2/authorization"
	tokenURL = "https://www.linkedin.com/oauth/v2/accessToken"
	apiBase  = "https://api.linkedin.com/v2"
)

type LinkedInClient struct {
	clientID     string
	clientSecret string
	accessToken  string
	httpClient   *http.Client
}

func NewLinkedInClient() *LinkedInClient {
	return &LinkedInClient{
		clientID:     os.Getenv("LINKEDIN_CLIENT_ID"),
		clientSecret: os.Getenv("LINKEDIN_CLIENT_SECRET"),
		accessToken:  os.Getenv("LINKEDIN_ACCESS_TOKEN"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *LinkedInClient) Close() {}

func (c *LinkedInClient) checkConfigured() error {
	if c.accessToken == "" {
		return fmt.Errorf("LINKEDIN_ACCESS_TOKEN not set — use linkedin_get_auth_url to start OAuth flow, then linkedin_exchange_code to get a token")
	}
	return nil
}

func (c *LinkedInClient) checkApp() error {
	if c.clientID == "" || c.clientSecret == "" {
		return fmt.Errorf("LINKEDIN_CLIENT_ID and LINKEDIN_CLIENT_SECRET must be set")
	}
	return nil
}

// --- OAuth helpers ---

func (c *LinkedInClient) GetAuthURL(redirectURI string, scopes []string) (string, error) {
	if err := c.checkApp(); err != nil {
		return "", err
	}
	if len(scopes) == 0 {
		scopes = []string{"openid", "profile", "email", "w_member_social", "r_organization_social", "w_organization_social", "rw_organization_admin"}
	}
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", c.clientID)
	params.Set("redirect_uri", redirectURI)
	params.Set("scope", strings.Join(scopes, " "))
	params.Set("state", fmt.Sprintf("%d", time.Now().UnixMilli()))
	return authURL + "?" + params.Encode(), nil
}

func (c *LinkedInClient) ExchangeCode(code, redirectURI string) (map[string]interface{}, error) {
	if err := c.checkApp(); err != nil {
		return nil, err
	}
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", c.clientID)
	form.Set("client_secret", c.clientSecret)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed HTTP %d: %s", resp.StatusCode, string(data))
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// --- HTTP helpers ---

func (c *LinkedInClient) newRequest(method, path string, body []byte) (*http.Request, error) {
	u := apiBase + path
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("X-RestLi-Protocol-Version", "2.0.0")
	req.Header.Set("LinkedIn-Version", "202401")
	req.Header.Set("Accept", "application/json")
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
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("access token expired or invalid — re-run linkedin_exchange_code to get a new token")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, truncateStr(string(data), 400))
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

var errNotAvailable = fmt.Errorf("this operation requires LinkedIn Partner Program access (Messaging/Connections API) — not available via standard OAuth2")

// --- Profile ---

func (c *LinkedInClient) GetMe() (json.RawMessage, error) {
	data, err := c.get("/me")
	return json.RawMessage(data), err
}

// --- Company ---

func (c *LinkedInClient) GetCompany(companyID string) (json.RawMessage, error) {
	path := fmt.Sprintf("/organizations/%s", url.PathEscape(companyID))
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
	data, err := c.post("/ugcPosts", content)
	return json.RawMessage(data), err
}

// --- Messaging (Partner API only) ---

func (c *LinkedInClient) ListConversations(_ int) (json.RawMessage, error) {
	return nil, errNotAvailable
}
func (c *LinkedInClient) GetConversation(_ string, _ int) (json.RawMessage, error) {
	return nil, errNotAvailable
}
func (c *LinkedInClient) SendMessage(_, _ string) (json.RawMessage, error) {
	return nil, errNotAvailable
}
func (c *LinkedInClient) NewConversation(_, _ string) (json.RawMessage, error) {
	return nil, errNotAvailable
}

// --- Connections (Partner API only) ---

func (c *LinkedInClient) ListConnections(_, _ int) (json.RawMessage, error) {
	return nil, errNotAvailable
}
func (c *LinkedInClient) SendConnectionRequest(_, _ string) (json.RawMessage, error) {
	return nil, errNotAvailable
}
func (c *LinkedInClient) ListPendingRequests(_ int) (json.RawMessage, error) {
	return nil, errNotAvailable
}
func (c *LinkedInClient) WithdrawRequest(_, _ string) error { return errNotAvailable }
func (c *LinkedInClient) AcceptRequest(_, _ string) error   { return errNotAvailable }

// --- Content ---

func (c *LinkedInClient) GetFeed(_ int) (json.RawMessage, error) {
	return nil, errNotAvailable
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
	data, err := c.post("/ugcPosts", payload)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) LikePost(activityURN string) error {
	payload := map[string]interface{}{
		"actor":     "",
		"object":    activityURN,
		"reactionType": "LIKE",
	}
	_, err := c.post(fmt.Sprintf("/socialActions/%s/likes", url.PathEscape(activityURN)), payload)
	return err
}

func (c *LinkedInClient) CommentOnPost(activityURN, text string) (json.RawMessage, error) {
	payload := map[string]interface{}{
		"actor":  "",
		"object": activityURN,
		"message": map[string]interface{}{
			"text": text,
		},
	}
	path := fmt.Sprintf("/socialActions/%s/comments", url.PathEscape(activityURN))
	data, err := c.post(path, payload)
	return json.RawMessage(data), err
}

func (c *LinkedInClient) SearchPosts(_ string, _ int) (json.RawMessage, error) {
	return nil, errNotAvailable
}

// --- Helpers ---

func generateTrackingID() string {
	return fmt.Sprintf("%d", time.Now().UnixMilli())
}

func extractCookieValue(cookieStr, name string) string {
	for _, part := range strings.Split(cookieStr, ";") {
		part = strings.TrimSpace(part)
		idx := strings.IndexByte(part, '=')
		if idx < 0 {
			continue
		}
		if strings.TrimSpace(part[:idx]) == name {
			return strings.TrimSpace(part[idx+1:])
		}
	}
	return ""
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
