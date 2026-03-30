# Installation

ServeRay MCP supports three install paths. Pick the simplest one that matches your environment.

## 1. Install a Single Server with Go

This is the easiest path for developers who already have Go installed.

Examples:

```bash
go install github.com/erayyal/serveray-mcp/cmd/mssql-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/postgres-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/mysql-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/github-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/jira-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/filesystem-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/slack-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/playwright-mcp@latest
```

That installs only the selected binary.

These commands assume the module path in `go.mod` matches the published GitHub repository path. If the repository is published somewhere else, update the module path and the install commands together.

## 2. Download a Release Binary

For users who do not want a Go toolchain, publish GitHub Releases from this repository.

The repository already includes `.goreleaser.yml`, so each server can be published as a standalone archive for:

- macOS
- Linux
- Windows

After extraction, the user only needs:

1. the binary
2. checksum verification
3. the relevant environment variables
4. an MCP client config entry

## 3. Clone and Install One Server from Source

```bash
git clone https://github.com/erayyal/serveray-mcp.git
cd serveray-mcp
make install-server SERVER=mssql-mcp
```

This installs a single server binary into `~/.local/bin` by default.

## Local Packaging

You can package a single server locally:

```bash
make package SERVER=mssql-mcp
```

Or package everything with GoReleaser:

```bash
goreleaser release --snapshot --clean
```

## Recommended Order for End Users

1. install one server only
2. copy that server’s `.env.example`
3. keep permissions and scopes minimal
4. verify the server starts cleanly over stdio
5. add the binary name to your MCP client config
