# playwright-mcp

`playwright-mcp` is a local stdio MCP server for constrained browser navigation with explicit origin allowlists. It keeps the repository Go-first by using `chromedp` under the `playwright-mcp` binary name.

## Required Permissions

- access only to the allowlisted sites you intentionally configure
- a local Chrome or Chromium-compatible browser

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/playwright-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=playwright-mcp
```

Run it locally after setting environment variables:

```bash
playwright-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- `PLAYWRIGHT_ALLOWED_ORIGINS` is mandatory
- `PLAYWRIGHT_ENABLE_SCREENSHOTS=false` is the safe default

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "playwright": {
      "command": "playwright-mcp",
      "env": {
        "PLAYWRIGHT_ALLOWED_ORIGINS": "https://example.com,https://docs.example.com"
      }
    }
  }
}
```

## Tools and Risk Levels

| Tool | Risk | Notes |
| --- | --- | --- |
| `fetch_page` | medium | Allowlisted navigation with bounded text extraction |
| `extract_links` | medium | Allowlisted navigation with bounded link extraction |
| `capture_screenshot` | high | Hidden unless screenshots are explicitly enabled |

## Safe Mode Behavior

- only allowlisted origins are reachable
- URLs with embedded credentials are rejected
- arbitrary script execution is not exposed
- downloads and uploads are intentionally unsupported
- screenshots stay disabled unless explicitly enabled
- navigation and extraction are bounded by time and output limits

## Optional Write Mode Behavior

Write mode is intentionally not implemented in this release.

## Limits and Timeouts

- navigation timeout
- bounded extracted text
- bounded extracted links
- optional screenshots only when explicitly enabled

## Known Limitations

- a local browser installation is still required
- browser navigation can still trigger normal site-side behavior inside the allowlist
- this is a constrained browser server, not a full interactive Playwright replacement

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `fetch_page` for an allowlisted URL and confirm it succeeds
- call `fetch_page` for a non-allowlisted URL and confirm it is rejected
- call `extract_links` and confirm the result is bounded
- if screenshots are disabled, confirm `capture_screenshot` is not exposed
