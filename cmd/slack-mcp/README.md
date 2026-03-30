# slack-mcp

`slack-mcp` is a local stdio MCP server for read-only Slack history access inside an explicit channel allowlist.

## Required Permissions

- the smallest history scopes that match your target channel types
- bot membership in the channels you want to read

Typical minimum scopes:

- `channels:read`
- `groups:read`
- `channels:history`
- `groups:history`

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/slack-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=slack-mcp
```

Run it locally after setting environment variables:

```bash
slack-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- `SLACK_ALLOWED_CHANNEL_IDS` is mandatory and uses exact channel IDs
- `SLACK_ALLOW_PRIVATE_HOSTS=false` and `SLACK_ALLOW_INSECURE_HTTP=false` are safe defaults
- only enable private hosts or insecure HTTP when you explicitly need a trusted proxy path

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "slack": {
      "command": "slack-mcp",
      "env": {
        "SLACK_BOT_TOKEN": "xoxb-replace-me",
        "SLACK_ALLOWED_CHANNEL_IDS": "C0123456789,C0987654321"
      }
    }
  }
}
```

## Tools and Risk Levels

| Tool | Risk | Notes |
| --- | --- | --- |
| `list_allowed_channels` | low | Returns configured channel IDs only |
| `channel_history` | medium | Remote read inside an allowlisted channel |
| `get_thread` | medium | Remote read inside an allowlisted channel thread |

## Safe Mode Behavior

- only channel IDs listed in `SLACK_ALLOWED_CHANNEL_IDS` are accessible
- no posting or mutation tools are exposed
- `list_allowed_channels` returns the configured allowlist without extra Slack discovery requests
- outbound requests are bounded, rate-limited, retried only for safe GET behavior, and protected by redirect and SSRF guardrails
- insecure HTTP and private/local API hosts are blocked by default

## Optional Write Mode Behavior

Write tools are intentionally not implemented in this release.

## Limits and Timeouts

- request timeout
- minimum interval between remote requests
- bounded message count
- truncated message text in responses
- bounded response body size

## Known Limitations

- channel allowlists are exact ID matches
- private channel access still depends on bot membership and Slack scopes
- user identity resolution remains intentionally minimal in this release

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `list_allowed_channels` and confirm it returns only configured IDs
- call `channel_history` for an allowlisted channel and confirm it succeeds
- call `channel_history` for a non-allowlisted channel and confirm it is rejected
- if Slack returns `not_in_channel`, invite the bot and confirm the read path then succeeds
