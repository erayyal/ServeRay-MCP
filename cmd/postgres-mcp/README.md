# postgres-mcp

`postgres-mcp` is a local stdio MCP server for PostgreSQL metadata inspection and bounded read queries with safe mode enabled by default.

## Required Permissions

- preferred: a dedicated read-only PostgreSQL role
- recommended: keep the backend role read-only even if the MCP write tool is never exposed

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/postgres-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=postgres-mcp
```

Run it locally after setting environment variables:

```bash
postgres-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- prefer structured `POSTGRES_HOST` / `POSTGRES_PORT` / `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DATABASE` over a raw DSN
- `POSTGRES_ENABLE_WRITE=false` is the safe default
- `POSTGRES_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS` is required before the write tool appears

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "postgres": {
      "command": "postgres-mcp",
      "env": {
        "POSTGRES_HOST": "localhost",
        "POSTGRES_PORT": "5432",
        "POSTGRES_USER": "readonly_user",
        "POSTGRES_PASSWORD": "replace-me",
        "POSTGRES_DATABASE": "app_db",
        "POSTGRES_SSLMODE": "require"
      }
    }
  }
}
```

## Tools and Risk Levels

| Tool | Risk | Notes |
| --- | --- | --- |
| `list_databases` | low | Metadata only |
| `list_schemas` | low | Metadata only |
| `list_tables` | low | Metadata only |
| `describe_table` | low | Metadata only |
| `query` | medium | Single read-only statement only |
| `execute_write` | high | Hidden unless explicit unsafe opt-in is enabled |

## Safe Mode Behavior

- `query` only accepts one read-only statement
- multi-statement input is blocked
- write keywords are blocked in safe mode
- structured DSN generation enables `default_transaction_read_only=on`
- per-query sessions apply PostgreSQL `statement_timeout`, `lock_timeout`, and idle transaction timeout guards
- rows, bytes, and cell values are capped before results are returned

## Optional Write Mode Behavior

`execute_write` appears only when both of these are set:

- `POSTGRES_ENABLE_WRITE=true`
- `POSTGRES_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS`

Even then, `EXEC`, `GRANT`, `REVOKE`, and multi-statement input remain blocked.

## Limits and Timeouts

- connection timeout
- query timeout
- bounded pool sizes
- connection lifetime and idle lifetime recycling
- row cap
- byte cap
- cell truncation

## Known Limitations

- a user-supplied DSN can override some structured safety defaults
- the SQL blocker is intentionally conservative and does not parse full SQL semantics
- database-side permissions still matter more than server-side guardrails

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `list_schemas` and confirm metadata returns normally
- call `query` with `SELECT now()` and confirm it succeeds
- call `query` with `UPDATE users SET active = false` and confirm it is rejected
- call `query` with `SELECT 1; DELETE FROM users` and confirm it is rejected
- if write mode is intentionally enabled, confirm `execute_write` appears and `REVOKE SELECT ON users FROM public` still fails
