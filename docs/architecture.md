# Architecture

## Goals

- keep each MCP server independently runnable
- reuse only true cross-cutting concerns
- make security controls obvious from code and documentation
- keep the repository easy to build with standard Go tooling
- keep per-server distribution simple for end users

## Module Strategy

The repository uses a single root module.

Why:

- one dependency graph
- one `go test ./...`
- simpler CI
- less risk of per-server version drift

## Layout

- `cmd/<server>-mcp`
  - binary entrypoints
- `internal/<server>`
  - domain-specific server logic, configuration, and tool handlers
- `internal/shared`
  - config loading, redaction, logging, SQL guardrails, HTTP client safety, filesystem enforcement, and MCP bootstrap helpers
- `docs`
  - architecture, security model, development guides
- `examples`
  - client configuration guidance
- `tests`
  - smoke and manual verification guidance

## Shared Packages

- `internal/shared/config`
  - environment parsing helpers
- `internal/shared/redaction`
  - secret scrubbing for logs and error messages
- `internal/shared/logging`
  - structured stderr logging with redaction-aware attributes
- `internal/shared/sqlsafe`
  - SQL input validation and multi-statement blocking
- `internal/shared/db`
  - connection pool defaults, result shaping, and reusable DB MCP tool registration
- `internal/shared/httpx`
  - bounded safe GET requests with retries, rate limiting, redirect checks, and SSRF guardrails
- `internal/shared/fsguard`
  - root allowlists, symlink handling, and traversal prevention
- `internal/shared/mcpserver`
  - server bootstrap, audit middleware, panic recovery, and static prompt/resource helpers

## Server Pattern

Each server package follows the same high-level shape:

1. load config from environment
2. validate allowlists and safe-mode toggles
3. build the server with shared middleware
4. register server-owned tools
5. expose a capabilities resource and a safe-usage prompt
6. serve over stdio from `cmd/<server>-mcp`

## Versioning Strategy

- the repo releases as a single semantic version
- binaries inherit the repo version
- new servers join the existing monorepo instead of becoming separate modules
- per-server README files stay local to `cmd/<server>-mcp` so download-and-run usage stays obvious

## Transport

The default transport is stdio.

That keeps deployment simple and avoids accidentally exposing a network listener. If HTTP transport is ever added later, it should bind to localhost by default, validate `Origin`, and require authentication.
