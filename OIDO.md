# OIDO LinkedIn Extension

Manage LinkedIn company pages, messaging, connections, and content via MCP tools.

## Setup

1. Open LinkedIn in your browser and log in
2. Open DevTools (F12) → Network tab → click any `linkedin.com` request
3. Find the `Cookie:` request header → copy the entire value
4. Paste it as `LINKEDIN_COOKIE` in extension settings

> **Note:** The cookie contains your `li_at` session token and `JSESSIONID` (used as CSRF token). It expires when you log out or LinkedIn rotates it — refresh it if tools start returning 401 errors.

## Available Tools

### Company
| Tool | Description |
|---|---|
| `linkedin_get_company` | Get company profile and stats by universal name or ID |
| `linkedin_post_as_company` | Create a post on a company page |

### Messaging
| Tool | Description |
|---|---|
| `linkedin_list_conversations` | List inbox conversations |
| `linkedin_get_conversation` | Get messages in a conversation thread |
| `linkedin_send_message` | Send a message in an existing conversation |
| `linkedin_new_conversation` | Start a new DM with someone |

### Connections
| Tool | Description |
|---|---|
| `linkedin_list_connections` | List your connections (most recent first) |
| `linkedin_send_connection_request` | Send a connection request with optional note |
| `linkedin_list_pending_requests` | List sent/received pending requests |
| `linkedin_withdraw_request` | Withdraw a sent request |
| `linkedin_accept_request` | Accept a received request |

### Content
| Tool | Description |
|---|---|
| `linkedin_get_feed` | Get your home feed posts |
| `linkedin_create_post` | Create a public post as yourself |
| `linkedin_like_post` | Like a post by activity URN |
| `linkedin_comment_post` | Comment on a post |
| `linkedin_search_posts` | Search posts by keyword |

## Finding URNs

- **Member URN**: visible in profile API responses as `entityUrn` → `urn:li:member:123456`
- **Company URN**: `urn:li:organization:1035`
- **Activity URN**: `urn:li:activity:7123456789` — visible in post URLs and feed responses
- **Conversation ID**: returned by `linkedin_list_conversations`

## Example Usage

```
User: "Post a company update about our new product"
→ linkedin_post_as_company with company_urn and text

User: "Show my unread messages"
→ linkedin_list_conversations

User: "Send a connection request to John with a note"
→ linkedin_send_connection_request with profile_urn and message

User: "Search for posts about AI agents"
→ linkedin_search_posts with keywords="AI agents"
```

## When to Use

- Managing LinkedIn company presence and posting
- Sending/reading direct messages
- Growing your network with connection requests
- Monitoring and engaging with LinkedIn content
