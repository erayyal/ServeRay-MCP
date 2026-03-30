# jira-mcp

`jira-mcp` is a local stdio MCP server for read-only issue lookup inside an explicit Jira project allowlist.

## Required Permissions

- browse access to the projects you allowlist
- the minimum token or user access needed to read issues, statuses, assignees, and summaries

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/jira-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=jira-mcp
```

Run it locally after setting environment variables:

```bash
jira-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- `JIRA_ALLOWED_PROJECTS` is mandatory
- `JIRA_ALLOW_PRIVATE_HOSTS=false` and `JIRA_ALLOW_INSECURE_HTTP=false` are safe defaults
- only enable private hosts or insecure HTTP when you explicitly need a trusted self-hosted Jira deployment

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "jira": {
      "command": "jira-mcp",
      "env": {
        "JIRA_BASE_URL": "https://your-domain.atlassian.net",
        "JIRA_EMAIL": "user@example.com",
        "JIRA_API_TOKEN": "replace-me",
        "JIRA_ALLOWED_PROJECTS": "APP,OPS"
      }
    }
  }
}
```

## Tools and Risk Levels

| Tool | Risk | Notes |
| --- | --- | --- |
| `list_allowed_projects` | low | Returns configured allowlist only |
| `search_issues` | medium | Structured remote read inside an allowlisted project |
| `get_issue` | medium | Remote read inside an allowlisted project |

## Safe Mode Behavior

- only project keys listed in `JIRA_ALLOWED_PROJECTS` are accessible
- the server builds JQL from structured filters instead of exposing arbitrary JQL
- outbound requests are bounded, rate-limited, retried only for safe GET behavior, and protected by redirect and SSRF guardrails
- insecure HTTP and private/local API hosts are blocked by default

## Optional Write Mode Behavior

Write tools are intentionally not implemented in this release.

## Limits and Timeouts

- request timeout
- minimum interval between remote requests
- bounded item counts
- bounded response body size

## Known Limitations

- the server covers a narrow Jira read surface on purpose
- custom or complex Jira search workflows may need a separate dedicated integration
- project allowlists still do not replace backend-side permission checks

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `list_allowed_projects` and confirm it returns the configured list
- call `search_issues` for an allowlisted project and confirm it succeeds
- call `get_issue` for a non-allowlisted project key and confirm it is rejected
- if using self-hosted Jira, verify private-host or insecure-http opt-ins are only enabled when truly needed
