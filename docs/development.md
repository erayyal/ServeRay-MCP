# Development

## Prerequisites

- Go 1.25 or newer installed and available on `PATH`
- optional `golangci-lint`
- optional `govulncheck`
- optional `goreleaser`
- backend-specific credentials only when testing a given server

## Common Commands

```bash
make fmt
make fmt-check
make vet
make test
make build
make verify
make lint
make vuln
make build-server SERVER=mssql-mcp
make install-server SERVER=mssql-mcp
make package SERVER=mssql-mcp
```

## Build a Single Server

```bash
go build ./cmd/mssql-mcp
go build ./cmd/postgres-mcp
go build ./cmd/mysql-mcp
go build ./cmd/github-mcp
go build ./cmd/jira-mcp
go build ./cmd/filesystem-mcp
go build ./cmd/slack-mcp
go build ./cmd/playwright-mcp
```

## Run a Server Locally

Use the server-specific `.env.example`, then:

```bash
go run ./cmd/mssql-mcp
```

The same pattern works for every server under `cmd/`.

## Release and Versioning

- repo tags use semantic versioning like `v0.1.0`
- every server binary shares the same repo version
- update `CHANGELOG.md` for public-facing changes
- run `make verify`, `make lint`, and `make vuln` before tagging when those tools are available

## Stdout Discipline

For stdio MCP servers:

- stdout is reserved for MCP messages only
- logs must stay on stderr
- never print startup banners or debug lines to stdout

## Manual Smoke Testing

See [tests/smoke/README.md](../tests/smoke/README.md) for MCP Inspector steps.
