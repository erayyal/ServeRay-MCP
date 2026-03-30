# Security Model

## Principles

- safe by default
- least privilege
- bounded execution
- explicit opt-in for unsafe behavior
- no secret leakage in normal logs
- local stdio first

## Tool Risk Levels

ServeRay MCP uses three documentation-only risk levels:

- `low`
  metadata lookup, bounded reads, allowlist listing
- `medium`
  remote API reads, browser navigation, file reads inside enforced roots
- `high`
  optional write tools or anything that can mutate external state

Every server README includes a per-tool risk table. High-risk tools must never appear by default.

## Database Servers

Database servers enforce:

- connection timeout
- query timeout via context
- small connection pools
- connection lifetime recycling
- row caps
- byte caps
- truncated cell values
- blocked multi-statement input
- blocked dangerous SQL in safe mode
- server-side session guards where supported

Write tools are hidden by default and only appear when both of these are set:

- `<PREFIX>_ENABLE_WRITE=true`
- `<PREFIX>_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS`

Even in write mode, `EXEC`, `EXECUTE`, `xp_cmdshell`, `GRANT`, `REVOKE`, and multi-statement input remain blocked.

## Filesystem Server

Filesystem access is constrained by:

- explicit root allowlists
- alias-based root selection
- relative-path-only input
- canonical path resolution
- symlink escape prevention
- hidden and sensitive path blocking by default

## Remote API Servers

GitHub, Jira, and Slack servers do not expose generic “call any endpoint” tools.

Instead they expose narrow read-only tools and validate:

- repository allowlists
- project allowlists
- channel allowlists
- bounded result counts
- safe request retries
- SSRF guardrails for outbound base URLs
- explicit opt-in for private hosts or insecure HTTP when an operator truly needs them

## Browser Server

The browser server is implemented in Go with `chromedp` to keep the repository Go-first. It enforces:

- allowlisted origins
- navigation timeouts
- bounded text extraction
- optional screenshots hidden by default
- no arbitrary script execution tool
- no upload or download tool

## Logging and Errors

- logs are structured
- logs go to stderr
- secrets are redacted
- tool invocations are auditable
- panic recovery returns safe errors instead of raw stack traces
- user-facing errors are concise and operationally safe

## Operational Responsibilities

Users still need to provide:

- low-privilege credentials
- isolated environments
- backend-side permissions
- secret management
- monitoring appropriate to their environment
