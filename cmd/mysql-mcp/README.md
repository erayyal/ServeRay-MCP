# mysql-mcp

`mysql-mcp` is a local stdio MCP server for MySQL metadata inspection and bounded read queries with safe mode enabled by default.

## Required Permissions

- preferred: a dedicated read-only MySQL account for the target schema
- if write mode is intentionally enabled, keep grants scoped to the smallest practical schema set

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/mysql-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=mysql-mcp
```

Run it locally after setting environment variables:

```bash
mysql-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- prefer structured `MYSQL_HOST` / `MYSQL_PORT` / `MYSQL_USER` / `MYSQL_PASSWORD` / `MYSQL_DATABASE` over a raw DSN
- `MYSQL_ENABLE_WRITE=false` is the safe default
- `MYSQL_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS` is required before the write tool appears

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "mysql": {
      "command": "mysql-mcp",
      "env": {
        "MYSQL_HOST": "localhost",
        "MYSQL_PORT": "3306",
        "MYSQL_USER": "readonly_user",
        "MYSQL_PASSWORD": "replace-me",
        "MYSQL_DATABASE": "app_db",
        "MYSQL_TLS": "true"
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
- `DROP`, `TRUNCATE`, `ALTER`, `CREATE`, `INSERT`, `UPDATE`, `DELETE`, `MERGE`, `GRANT`, `REVOKE`, `EXEC`, and `EXECUTE` are blocked in safe mode
- rows, bytes, and cell values are capped before results are returned
- the server favors metadata tools and narrow read queries over large scans

## Optional Write Mode Behavior

`execute_write` appears only when both of these are set:

- `MYSQL_ENABLE_WRITE=true`
- `MYSQL_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS`

Even then, multi-statement input and `EXEC`-style commands remain blocked.

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
- safe mode is intentionally conservative and may reject queries that are technically read-only but unusual
- database-side permissions still matter more than server-side guardrails

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `describe_table` and confirm metadata returns normally
- call `query` with `SELECT 1` and confirm it succeeds
- call `query` with `TRUNCATE TABLE demo` and confirm it is rejected
- call `query` with `SELECT 1; DELETE FROM demo` and confirm it is rejected
- if write mode is intentionally enabled, confirm `execute_write` appears and `EXECUTE IMMEDIATE 'DROP TABLE demo'` still fails
