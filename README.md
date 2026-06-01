# oido-linkedin

LinkedIn MCP plugin for [Oido](https://oido.ai). Manage company pages, messaging, connections, and content via AI tools.

## Features

- **Company** — read company profiles, post as company page
- **Messaging** — list conversations, read threads, send messages, start new DMs
- **Connections** — list connections, send/accept/withdraw requests
- **Content** — read feed, create posts, like and comment

## Auth

Uses your browser session cookie (`li_at` + `JSESSIONID`). No OAuth app required.

1. Log in to LinkedIn in your browser
2. Open DevTools → Network → any `linkedin.com` request → copy the `Cookie:` header
3. Set it as `LINKEDIN_COOKIE` in the plugin settings

## Build

```bash
make build
```

## Tools

| Tool | Description |
|---|---|
| `linkedin_get_company` | Get company profile |
| `linkedin_post_as_company` | Post as company page |
| `linkedin_list_conversations` | List inbox |
| `linkedin_get_conversation` | Read message thread |
| `linkedin_send_message` | Send message |
| `linkedin_new_conversation` | Start new DM |
| `linkedin_list_connections` | List connections |
| `linkedin_send_connection_request` | Send connection request |
| `linkedin_list_pending_requests` | List pending requests |
| `linkedin_withdraw_request` | Withdraw request |
| `linkedin_accept_request` | Accept request |
| `linkedin_get_feed` | Read home feed |
| `linkedin_create_post` | Create post |
| `linkedin_like_post` | Like a post |
| `linkedin_comment_post` | Comment on a post |
| `linkedin_search_posts` | Search posts |

## Disclaimer

This plugin uses LinkedIn's internal Voyager API via session cookies. Use responsibly and in accordance with LinkedIn's Terms of Service.
