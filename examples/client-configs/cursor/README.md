# Cursor

Use the same `command` plus `env` model shown in each server README and place it in Cursor's MCP configuration.

```json
{
  "mcpServers": {
    "github": {
      "command": "github-mcp",
      "env": {
        "GITHUB_TOKEN": "replace-me",
        "GITHUB_ALLOWED_REPOS": "owner/repo"
      }
    }
  }
}
```

If the binary is not on `PATH`, use an absolute path instead.
