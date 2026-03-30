# Changelog

All notable changes to ServeRay MCP will be documented in this file.

The repository follows Semantic Versioning.

## [Unreleased]

### Added
- Public-release hardening for all local stdio MCP servers.
- Shared panic recovery, stderr-only structured logging, secret redaction, and outbound HTTP SSRF guards.
- Release packaging, checksum generation, `govulncheck`, smoke-test guidance, and normalized server documentation.

## [0.1.0] - 2026-03-29

### Added
- Initial public ServeRay MCP monorepo release with:
  - `mssql-mcp`
  - `postgres-mcp`
  - `mysql-mcp`
  - `github-mcp`
  - `jira-mcp`
  - `filesystem-mcp`
  - `slack-mcp`
  - `playwright-mcp`
