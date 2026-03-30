# mssql-mcp

`mssql-mcp` is a local stdio MCP server for Microsoft SQL Server metadata inspection and bounded read queries with safe mode enabled by default.

## Required Permissions

- preferred: a dedicated read-only SQL Server login for the target database
- if write mode is intentionally enabled, grant only the smallest role needed for that specific workflow

## Install and Run

Install only this server with Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/mssql-mcp@latest
```

Install from a clone:

```bash
make install-server SERVER=mssql-mcp
```

Run it locally after setting environment variables:

```bash
mssql-mcp
```

Release archives include the binary, this README, `.env.example`, and a checksum file.

## Environment Variables

See [`./.env.example`](./.env.example).

Notes:

- prefer structured `MSSQL_HOST` / `MSSQL_PORT` / `MSSQL_USER` / `MSSQL_PASSWORD` / `MSSQL_DATABASE` over a raw DSN
- `MSSQL_ENABLE_WRITE=false` is the safe default
- `MSSQL_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS` is required before the write tool appears

## Sample MCP Client Config

```json
{
  "mcpServers": {
    "mssql": {
      "command": "mssql-mcp",
      "env": {
        "MSSQL_HOST": "localhost",
        "MSSQL_PORT": "1433",
        "MSSQL_USER": "readonly_user",
        "MSSQL_PASSWORD": "replace-me",
        "MSSQL_DATABASE": "app_db"
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
- `DROP`, `TRUNCATE`, `ALTER`, `CREATE`, `INSERT`, `UPDATE`, `DELETE`, `MERGE`, `GRANT`, `REVOKE`, `EXEC`, `EXECUTE`, and `xp_cmdshell` are blocked
- query sessions use low deadlock priority, `LOCK_TIMEOUT`, and `READ UNCOMMITTED`
- rows, bytes, and cell values are capped before returning results to the client

## Optional Write Mode Behavior

`execute_write` appears only when both of these are set:

- `MSSQL_ENABLE_WRITE=true`
- `MSSQL_WRITE_ACK=ENABLE_UNSAFE_WRITE_OPERATIONS`

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

- a user-supplied DSN can override some structured defaults
- safe mode is a guardrail, not a substitute for least-privileged SQL permissions
- driver and server behavior still control the exact cancellation timing of long queries

## Manual Verification Checklist

- start the server and confirm it writes logs only to `stderr`
- call `list_tables` and confirm metadata returns normally
- call `query` with `SELECT 1` and confirm it succeeds
- call `query` with `DROP TABLE demo` and confirm it is rejected
- call `query` with `SELECT 1; DELETE FROM demo` and confirm it is rejected
- if write mode is intentionally enabled, confirm `execute_write` appears and `EXEC xp_cmdshell 'dir'` still fails
