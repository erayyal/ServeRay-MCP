# Smoke Tests

## Goal

Validate that a server:

- starts over stdio
- responds to MCP initialization
- exposes the expected tools
- keeps stdout reserved for MCP traffic
- returns safe, bounded errors instead of panic output

## Example with MCP Inspector

Build the target binary first:

```bash
go build ./cmd/filesystem-mcp
```

Launch MCP Inspector with environment variables for the server you want to test, then configure the command path to the built binary.

Recommended checks:

1. initialize the server
2. list tools
3. call a safe read-only tool
4. confirm dangerous DB input is rejected in safe mode
5. confirm oversized outputs are bounded
6. confirm logs stay on `stderr`
7. confirm secrets do not appear in returned error text

## Manual DB Safety Checks

- call `query` with `DROP TABLE x`
- call `query` with `SELECT 1; DELETE FROM x`
- verify both return tool-level errors in safe mode
- if write mode is enabled on purpose, verify `EXEC`, `REVOKE`, and multi-statement input still fail

## Manual Filesystem Safety Checks

- request `../` traversal
- request a hidden path like `.ssh`
- verify both are rejected

## Manual Remote API Checks

- call an allowlisted GitHub repo and a non-allowlisted repo
- call an allowlisted Jira project and a non-allowlisted project
- call an allowlisted Slack channel and a non-allowlisted channel
- verify only allowlisted targets succeed

## Manual Browser Checks

- fetch an allowlisted URL
- fetch a non-allowlisted URL
- verify the non-allowlisted request is rejected
- if screenshots are disabled, confirm `capture_screenshot` is not exposed
