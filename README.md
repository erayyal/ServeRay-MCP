# ServeRay MCP

ServeRay MCP is a public Go monorepo of local-first MCP servers for users who want to install one server, configure it with their own credentials, and run it over stdio on their own machine.

This repository is not a hosted platform, not a shared control plane, and not a multi-tenant service. It is a collection of download-and-run MCP binaries with safe defaults, explicit boundaries, and documentation that treats security as an operational concern instead of a demo checkbox.

## Included Servers

| Server | Purpose | Default Mode | Write Support | Default Blockers | Status |
| --- | --- | --- | --- | --- | --- |
| `mssql-mcp` | Microsoft SQL Server metadata and bounded read queries | Safe read-only | Optional, explicit opt-in | Multi-statement SQL, writes, `EXEC`, `xp_cmdshell` | Beta |
| `postgres-mcp` | PostgreSQL metadata and bounded read queries | Safe read-only | Optional, explicit opt-in | Multi-statement SQL, writes in safe mode, lock-heavy sessions | Beta |
| `mysql-mcp` | MySQL metadata and bounded read queries | Safe read-only | Optional, explicit opt-in | Multi-statement SQL, writes in safe mode, unbounded results | Beta |
| `github-mcp` | Read-only issue and pull request access for allowlisted repositories | Read-only | No | Arbitrary endpoints, non-allowlisted repos, insecure/private base URLs by default | Beta |
| `jira-mcp` | Read-only issue lookup for allowlisted Jira projects | Read-only | No | Arbitrary JQL, non-allowlisted projects, insecure/private base URLs by default | Beta |
| `filesystem-mcp` | Read-only file inspection inside configured roots | Read-only | No | Absolute paths, traversal, hidden/sensitive paths by default | Beta |
| `slack-mcp` | Read-only Slack history for allowlisted channels | Read-only | No | Posting, non-allowlisted channels, unnecessary discovery requests, insecure/private base URLs by default | Beta |
| `playwright-mcp` | Constrained browser navigation with allowlisted origins | Allowlisted navigation only | No | Arbitrary browsing, script execution, uploads/downloads, screenshots by default | Beta |

## Compatibility

| Area | Current Support |
| --- | --- |
| Transport | Local stdio only |
| Build toolchain | Go 1.25+ |
| Release binaries | macOS, Linux, Windows |
| Logging | Structured JSON on `stderr` only |
| Configuration | Environment variables and per-server `.env.example` files |
| Versioning | Single repo-wide semantic version shared across binaries |

## Install One Server

Use `go install` if you already have Go:

```bash
go install github.com/erayyal/serveray-mcp/cmd/mssql-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/postgres-mcp@latest
go install github.com/erayyal/serveray-mcp/cmd/github-mcp@latest
```

If you prefer source installs from a clone:

```bash
git clone https://github.com/erayyal/serveray-mcp.git
cd serveray-mcp
make install-server SERVER=mssql-mcp
```

If you prefer release binaries, download the archive for the specific server you want, verify the checksum, put the binary on `PATH`, copy its `.env.example`, and run it locally.

First public release: [v0.1.0](https://github.com/erayyal/ServeRay-MCP/releases/tag/v0.1.0)

These install commands assume the published module path remains `github.com/erayyal/serveray-mcp`. If the final GitHub owner changes, update `go.mod`, README examples, and release notes together.

## First Run

1. Pick a single server under `cmd/<server>`.
2. Copy that server’s `.env.example`.
3. Replace placeholders with your own credentials, scopes, roots, or allowlists.
4. Keep permissions narrow:
   - use read-only database accounts
   - scope GitHub tokens to the exact repositories you need
   - scope Jira access to browse-only projects when possible
   - scope Slack access to the minimum history scopes
   - keep filesystem roots small and non-sensitive
   - keep browser origin allowlists narrow
5. Start the binary from your shell:

```bash
mssql-mcp
```

Example MCP client config:

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

## Safe-by-Default Philosophy

- Least privilege first. The server cannot make a backend safer than the credentials, scopes, or roots you give it.
- Read-only by default. Database write tools are hidden until an explicit enable flag and acknowledgement string are both set.
- Dangerous operations blocked in safe mode. Database safe mode blocks multi-statement execution and common destructive SQL verbs.
- Bounded outputs. Rows, bytes, file lines, API items, page text, and extracted links are capped.
- Server-side enforcement. Filesystem roots, repository/project/channel allowlists, and browser origin allowlists are enforced by the server, not just suggested by the client.
- Clean stdio transport. MCP messages stay on `stdout`; operational logs stay on `stderr`.

## What Is Blocked by Default

- Database writes, schema changes, and multi-statement input
- `EXEC`, `EXECUTE`, `xp_cmdshell`, `GRANT`, and `REVOKE`
- Filesystem traversal, absolute client paths, and hidden/sensitive paths
- Arbitrary GitHub/Jira/Slack endpoint access
- Insecure HTTP base URLs and private/local API hosts unless explicitly allowed
- Arbitrary browser automation, screenshots, uploads, and downloads

## Release and Versioning

- Repo tags follow Semantic Versioning: `vMAJOR.MINOR.PATCH`
- All binaries in this repo ship under the same repo version
- Breaking tool behavior or config changes require a major release
- New backward-compatible servers or tools use a minor release
- Fixes, documentation updates, and non-breaking hardening use a patch release
- Keep [`CHANGELOG.md`](./CHANGELOG.md) updated for public releases

## Repository Layout

```text
.
├── cmd/        # Binary entrypoints and per-server docs
├── docs/       # Architecture, security, install, and authoring guidance
├── examples/   # MCP client configuration examples
├── internal/   # Server implementations and shared guardrails
├── scripts/    # Transparent helper scripts
├── tests/      # Manual smoke guidance and test support
├── Makefile
├── README.md
└── SECURITY.md
```

The repo uses a single root `go.mod` on purpose. That keeps dependency management, CI, release packaging, and future server additions simpler than running one module per server.

## Per-Server Setup

- [MSSQL](./cmd/mssql-mcp/README.md)
- [PostgreSQL](./cmd/postgres-mcp/README.md)
- [MySQL](./cmd/mysql-mcp/README.md)
- [GitHub](./cmd/github-mcp/README.md)
- [Jira](./cmd/jira-mcp/README.md)
- [Filesystem](./cmd/filesystem-mcp/README.md)
- [Slack](./cmd/slack-mcp/README.md)
- [Playwright](./cmd/playwright-mcp/README.md)

## Docs

- [Installation](./docs/installation.md)
- [Architecture](./docs/architecture.md)
- [Security Model](./docs/security-model.md)
- [Development](./docs/development.md)
- [Adding a New Server](./docs/adding-a-new-server.md)
- [Server README Template](./docs/server-readme-template.md)

## Operator Responsibility

ServeRay MCP reduces risk. It does not remove operator responsibility.

You remain responsible for:

- backend permissions and network reachability
- credential and token storage
- repo/project/channel/origin allowlists
- infrastructure isolation
- audit and compliance requirements in your environment

Do not point these binaries at production systems with broader permissions than you are willing to expose to an automated MCP caller.
