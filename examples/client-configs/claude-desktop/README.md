# Claude Desktop

Place your MCP JSON in Claude Desktop's MCP config file and point the `command` field to the binary name on `PATH` when possible.

Each server README includes a ready-to-edit JSON snippet using the same shape:

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "filesystem-mcp",
      "env": {
        "FILESYSTEM_ALLOWED_ROOTS": "repo=/absolute/path/to/repo"
      }
    }
  }
}
```

If the binary is not on `PATH`, use an absolute path instead.
