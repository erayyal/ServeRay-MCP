# github-mcp

`github-mcp` is a local stdio MCP server for read-only GitHub issue and pull request access inside an explicit repository allowlist.

## Required Permissions

- a fine-grained token with read access only to the repositories you need
- minimum scopes should cover metadata, issues, and pull requests for those repositories

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/github-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=github-mcp
```

Run it locally after setting environment variables:

```bash
github-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- `GITHUB_ALLOWED_REPOS` is mandatory and uses exact `owner/name` entries
- `GITHUB_ALLOW_PRIVATE_HOSTS=false` and `GITHUB_ALLOW_INSECURE_HTTP=false` are safe defaults
- only enable private hosts or insecure HTTP when you explicitly need GitHub Enterprise or a trusted proxy

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "github": {
      "command": "github-mcp",
      "env": {
        "GITHUB_TOKEN": "replace-me",
        "GITHUB_ALLOWED_REPOS": "owner/repo-one,owner/repo-two"
      }
    }
  }
}
```

## Tools and Risk Levels

| Tool | Risk | Notes |
| --- | --- | --- |
| `list_allowed_repositories` | low | Returns configured allowlist only |
| `list_issues` | medium | Remote read against an allowlisted repo |
| `get_issue` | medium | Remote read against an allowlisted repo |
| `list_pull_requests` | medium | Remote read against an allowlisted repo |

## Safe Mode Behavior

- only repositories listed in `GITHUB_ALLOWED_REPOS` are accessible
- no arbitrary endpoint or arbitrary repository tool exists
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

- repository allowlists are exact `owner/name` matches
- the exposed GitHub surface is intentionally narrow
- token scope and repository membership are still the operator’s responsibility

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `list_allowed_repositories` and confirm it returns the configured list without extra API discovery
- call `list_issues` for an allowlisted repository and confirm it succeeds
- call `list_issues` for a non-allowlisted repository and confirm it is rejected
- if using GitHub Enterprise, verify private-host or insecure-http opt-ins are only enabled when truly needed
