package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCPHandler struct {
	client *LinkedInClient
}

func NewMCPHandler(client *LinkedInClient) *MCPHandler {
	return &MCPHandler{client: client}
}

// --- Arg types ---

type GetCompanyArgs struct {
	CompanyID string `json:"company_id" jsonschema:"Company universal name or numeric ID (e.g. 'microsoft' or '1035')"`
}

type PostAsCompanyArgs struct {
	CompanyURN string   `json:"company_urn" jsonschema:"Company URN (e.g. urn:li:organization:1035)"`
	Text       string   `json:"text"        jsonschema:"Post text content"`
	MediaURNs  []string `json:"media_urns,omitempty" jsonschema:"Optional list of uploaded media asset URNs"`
}

type ListConversationsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max conversations to return (default: 20)"`
}

type GetConversationArgs struct {
	ConversationID string `json:"conversation_id" jsonschema:"Conversation ID or URN"`
	Limit          int    `json:"limit,omitempty" jsonschema:"Max messages to return (default: 20)"`
}

type SendMessageArgs struct {
	ConversationID string `json:"conversation_id" jsonschema:"Conversation ID or URN"`
	Text           string `json:"text"            jsonschema:"Message text"`
}

type NewConversationArgs struct {
	RecipientURN string `json:"recipient_urn" jsonschema:"Recipient member URN (e.g. urn:li:member:123456)"`
	Text         string `json:"text"          jsonschema:"Initial message text"`
}

type ListConnectionsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max connections to return (default: 20)"`
	Start int `json:"start,omitempty" jsonschema:"Pagination offset (default: 0)"`
}

type SendConnectionRequestArgs struct {
	ProfileURN string `json:"profile_urn"      jsonschema:"Target profile URN or public ID (e.g. urn:li:fsd_profile:ACoAAA...)"`
	Message    string `json:"message,omitempty" jsonschema:"Optional personalised connection note (max 300 chars)"`
}

type ListPendingRequestsArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max pending requests to return (default: 20)"`
}

type InvitationActionArgs struct {
	InvitationID string `json:"invitation_id"  jsonschema:"Invitation ID"`
	SharedSecret string `json:"shared_secret"  jsonschema:"Shared secret from the invitation (returned with pending requests)"`
}

type GetFeedArgs struct {
	Limit int `json:"limit,omitempty" jsonschema:"Max feed posts to return (default: 20)"`
}

type CreatePostArgs struct {
	AuthorURN string `json:"author_urn" jsonschema:"Author URN — use your member URN (e.g. urn:li:member:123456)"`
	Text      string `json:"text"       jsonschema:"Post text content"`
}

type LikePostArgs struct {
	ActivityURN string `json:"activity_urn" jsonschema:"Post activity URN (e.g. urn:li:activity:7123456789)"`
}

type CommentOnPostArgs struct {
	ActivityURN string `json:"activity_urn" jsonschema:"Post activity URN"`
	Text        string `json:"text"         jsonschema:"Comment text"`
}

type SearchPostsArgs struct {
	Keywords string `json:"keywords"        jsonschema:"Search keywords"`
	Limit    int    `json:"limit,omitempty" jsonschema:"Max results (default: 20)"`
}

// --- Helpers ---

func errResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: "Error: " + msg}},
		IsError: true,
	}
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}
}

func jsonResult(v interface{}) *mcp.CallToolResult {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(err.Error())
	}
	return textResult(string(data))
}

func rawResult(data []byte) *mcp.CallToolResult {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return textResult(string(data))
	}
	return jsonResult(v)
}

// --- Company handlers ---

func (h *MCPHandler) HandleGetCompany(_ context.Context, _ *mcp.CallToolRequest, args GetCompanyArgs) (*mcp.CallToolResult, any, error) {
	if args.CompanyID == "" {
		return errResult("company_id is required"), nil, nil
	}
	data, err := h.client.GetCompany(args.CompanyID)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandlePostAsCompany(_ context.Context, _ *mcp.CallToolRequest, args PostAsCompanyArgs) (*mcp.CallToolResult, any, error) {
	if args.CompanyURN == "" {
		return errResult("company_urn is required"), nil, nil
	}
	if args.Text == "" {
		return errResult("text is required"), nil, nil
	}
	data, err := h.client.PostAsCompany(args.CompanyURN, args.Text, args.MediaURNs)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

// --- Messaging handlers ---

func (h *MCPHandler) HandleListConversations(_ context.Context, _ *mcp.CallToolRequest, args ListConversationsArgs) (*mcp.CallToolResult, any, error) {
	data, err := h.client.ListConversations(args.Limit)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleGetConversation(_ context.Context, _ *mcp.CallToolRequest, args GetConversationArgs) (*mcp.CallToolResult, any, error) {
	if args.ConversationID == "" {
		return errResult("conversation_id is required"), nil, nil
	}
	data, err := h.client.GetConversation(args.ConversationID, args.Limit)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleSendMessage(_ context.Context, _ *mcp.CallToolRequest, args SendMessageArgs) (*mcp.CallToolResult, any, error) {
	if args.ConversationID == "" {
		return errResult("conversation_id is required"), nil, nil
	}
	if args.Text == "" {
		return errResult("text is required"), nil, nil
	}
	data, err := h.client.SendMessage(args.ConversationID, args.Text)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleNewConversation(_ context.Context, _ *mcp.CallToolRequest, args NewConversationArgs) (*mcp.CallToolResult, any, error) {
	if args.RecipientURN == "" {
		return errResult("recipient_urn is required"), nil, nil
	}
	if args.Text == "" {
		return errResult("text is required"), nil, nil
	}
	data, err := h.client.NewConversation(args.RecipientURN, args.Text)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

// --- Connection handlers ---

func (h *MCPHandler) HandleListConnections(_ context.Context, _ *mcp.CallToolRequest, args ListConnectionsArgs) (*mcp.CallToolResult, any, error) {
	data, err := h.client.ListConnections(args.Limit, args.Start)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleSendConnectionRequest(_ context.Context, _ *mcp.CallToolRequest, args SendConnectionRequestArgs) (*mcp.CallToolResult, any, error) {
	if args.ProfileURN == "" {
		return errResult("profile_urn is required"), nil, nil
	}
	if len(args.Message) > 300 {
		return errResult(fmt.Sprintf("message too long: %d chars (max 300)", len(args.Message))), nil, nil
	}
	data, err := h.client.SendConnectionRequest(args.ProfileURN, args.Message)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleListPendingRequests(_ context.Context, _ *mcp.CallToolRequest, args ListPendingRequestsArgs) (*mcp.CallToolResult, any, error) {
	data, err := h.client.ListPendingRequests(args.Limit)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleWithdrawRequest(_ context.Context, _ *mcp.CallToolRequest, args InvitationActionArgs) (*mcp.CallToolResult, any, error) {
	if args.InvitationID == "" {
		return errResult("invitation_id is required"), nil, nil
	}
	if args.SharedSecret == "" {
		return errResult("shared_secret is required"), nil, nil
	}
	if err := h.client.WithdrawRequest(args.InvitationID, args.SharedSecret); err != nil {
		return errResult(err.Error()), nil, nil
	}
	return textResult(fmt.Sprintf("Invitation %s withdrawn.", args.InvitationID)), nil, nil
}

func (h *MCPHandler) HandleAcceptRequest(_ context.Context, _ *mcp.CallToolRequest, args InvitationActionArgs) (*mcp.CallToolResult, any, error) {
	if args.InvitationID == "" {
		return errResult("invitation_id is required"), nil, nil
	}
	if args.SharedSecret == "" {
		return errResult("shared_secret is required"), nil, nil
	}
	if err := h.client.AcceptRequest(args.InvitationID, args.SharedSecret); err != nil {
		return errResult(err.Error()), nil, nil
	}
	return textResult(fmt.Sprintf("Invitation %s accepted.", args.InvitationID)), nil, nil
}

// --- Content handlers ---

func (h *MCPHandler) HandleGetFeed(_ context.Context, _ *mcp.CallToolRequest, args GetFeedArgs) (*mcp.CallToolResult, any, error) {
	data, err := h.client.GetFeed(args.Limit)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleCreatePost(_ context.Context, _ *mcp.CallToolRequest, args CreatePostArgs) (*mcp.CallToolResult, any, error) {
	if args.AuthorURN == "" {
		return errResult("author_urn is required"), nil, nil
	}
	if args.Text == "" {
		return errResult("text is required"), nil, nil
	}
	data, err := h.client.CreatePost(args.AuthorURN, args.Text)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleLikePost(_ context.Context, _ *mcp.CallToolRequest, args LikePostArgs) (*mcp.CallToolResult, any, error) {
	if args.ActivityURN == "" {
		return errResult("activity_urn is required"), nil, nil
	}
	if err := h.client.LikePost(args.ActivityURN); err != nil {
		return errResult(err.Error()), nil, nil
	}
	return textResult(fmt.Sprintf("Liked post %s.", args.ActivityURN)), nil, nil
}

func (h *MCPHandler) HandleCommentOnPost(_ context.Context, _ *mcp.CallToolRequest, args CommentOnPostArgs) (*mcp.CallToolResult, any, error) {
	if args.ActivityURN == "" {
		return errResult("activity_urn is required"), nil, nil
	}
	if args.Text == "" {
		return errResult("text is required"), nil, nil
	}
	data, err := h.client.CommentOnPost(args.ActivityURN, args.Text)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

func (h *MCPHandler) HandleSearchPosts(_ context.Context, _ *mcp.CallToolRequest, args SearchPostsArgs) (*mcp.CallToolResult, any, error) {
	if args.Keywords == "" {
		return errResult("keywords is required"), nil, nil
	}
	data, err := h.client.SearchPosts(args.Keywords, args.Limit)
	if err != nil {
		return errResult(err.Error()), nil, nil
	}
	return rawResult(data), nil, nil
}

// --- Server bootstrap ---

func RunMCPServer() {
	client, err := NewLinkedInClient()
	if err != nil {
		log.Printf("Warning: LinkedIn client init failed: %v", err)
		client = &LinkedInClient{httpClient: nil}
	}

	handler := NewMCPHandler(client)

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "oido-linkedin",
		Version: "1.0.0",
	}, nil)

	// --- Company tools ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_get_company",
		Description: "Get LinkedIn company profile and stats by universal name or numeric ID.",
	}, handler.HandleGetCompany)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_post_as_company",
		Description: "Create a post on a LinkedIn company page. Optionally attach uploaded media URNs.",
	}, handler.HandlePostAsCompany)

	// --- Messaging tools ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_list_conversations",
		Description: "List LinkedIn message conversations (inbox). Returns conversation IDs, participants, and unread counts.",
	}, handler.HandleListConversations)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_get_conversation",
		Description: "Get messages in a LinkedIn conversation thread by conversation ID.",
	}, handler.HandleGetConversation)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_send_message",
		Description: "Send a message in an existing LinkedIn conversation.",
	}, handler.HandleSendMessage)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_new_conversation",
		Description: "Start a new LinkedIn DM conversation with a person by their member URN.",
	}, handler.HandleNewConversation)

	// --- Connection tools ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_list_connections",
		Description: "List your LinkedIn connections sorted by most recently connected.",
	}, handler.HandleListConnections)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_send_connection_request",
		Description: "Send a LinkedIn connection request to a profile. Optionally include a personalised note (max 300 chars).",
	}, handler.HandleSendConnectionRequest)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_list_pending_requests",
		Description: "List sent and received pending LinkedIn connection requests.",
	}, handler.HandleListPendingRequests)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_withdraw_request",
		Description: "Withdraw a sent LinkedIn connection request by invitation ID and shared secret.",
	}, handler.HandleWithdrawRequest)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_accept_request",
		Description: "Accept a received LinkedIn connection request by invitation ID and shared secret.",
	}, handler.HandleAcceptRequest)

	// --- Content tools ---
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_get_feed",
		Description: "Get your LinkedIn home feed posts in chronological order.",
	}, handler.HandleGetFeed)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_create_post",
		Description: "Create a public LinkedIn post as yourself.",
	}, handler.HandleCreatePost)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_like_post",
		Description: "Like a LinkedIn post by its activity URN.",
	}, handler.HandleLikePost)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_comment_post",
		Description: "Comment on a LinkedIn post by its activity URN.",
	}, handler.HandleCommentOnPost)
	mcp.AddTool(server, &mcp.Tool{
		Name:        "linkedin_search_posts",
		Description: "Search LinkedIn posts by keyword.",
	}, handler.HandleSearchPosts)

	ctx := context.Background()
	log.Println("Oido LinkedIn MCP Server starting on stdio...")
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("MCP server error: %v", err)
	}
}
