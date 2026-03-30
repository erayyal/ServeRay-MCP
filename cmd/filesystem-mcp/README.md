# filesystem-mcp

`filesystem-mcp` is a local stdio MCP server for bounded read-only file inspection inside explicit server-side allowlisted roots.

## Required Permissions

- read access to the directories you explicitly allowlist
- no extra permissions outside those roots

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/filesystem-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=filesystem-mcp
```

Run it locally after setting environment variables:

```bash
filesystem-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- `FILESYSTEM_ALLOWED_ROOTS` is mandatory and uses `alias=/absolute/path` entries
- `FILESYSTEM_ALLOW_HIDDEN=false` is the safe default

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "filesystem-mcp",
      "env": {
        "FILESYSTEM_ALLOWED_ROOTS": "repo=/absolute/path/to/repo,docs=/absolute/path/to/docs"
      }
    }
  }
}
```

## Tools and Risk Levels

| Tool | Risk | Notes |
| --- | --- | --- |
| `list_allowed_roots` | low | Returns configured aliases only |
| `list_directory` | medium | Reads directory metadata inside an enforced root |
| `read_text_file` | medium | Reads bounded text inside an enforced root |
| `stat_path` | low | Reads file or directory metadata only |

## Safe Mode Behavior

- access is limited to configured roots only
- client-supplied absolute paths are rejected
- path traversal is blocked
- hidden and sensitive paths are blocked by default
- symlink escapes are blocked
- write tools are intentionally absent

## Optional Write Mode Behavior

Write mode is intentionally not implemented in this release.

## Limits and Timeouts

- bounded directory entry counts
- bounded file bytes
- bounded line counts

## Known Limitations

- binary file handling is intentionally omitted
- root allowlists are enforced server-side, but OS permissions still apply
- hidden path access requires explicit configuration and should remain rare

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `list_allowed_roots` and confirm the configured aliases appear
- call `read_text_file` inside an allowlisted root and confirm it succeeds
- call `read_text_file` with `../` traversal and confirm it is rejected
- call `read_text_file` with a hidden path such as `.ssh/config` and confirm it is rejected unless explicitly allowed
