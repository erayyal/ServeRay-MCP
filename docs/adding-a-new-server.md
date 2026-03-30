# Adding a New Server

## Checklist

1. Create `cmd/<name>-mcp/main.go`.
2. Add `internal/<name>` for configuration and tool logic.
3. Reuse `internal/shared` only for genuine cross-cutting code.
4. Add:
   - server README
   - `.env.example`
   - capabilities resource
   - safe-usage prompt
   - safety-critical tests
   - tool risk table
   - manual verification checklist
5. Update:
   - root `README.md`
   - `CHANGELOG.md`
   - root `.env.example` if needed
   - `docs/architecture.md` when the shared shape changes
   - `docs/server-readme-template.md` if the standard README shape changes

## Design Rules

- default to read-only
- avoid generic raw endpoint callers
- validate and bound all user input
- keep stdout clean for MCP traffic
- redact secrets in all logs and errors
- use repo-wide semantic versioning
- only add private-host or insecure-http opt-ins when the server truly needs outbound HTTP

## When Shared Code Is Justified

Add shared code only if at least two servers need the same behavior and the abstraction stays simpler than duplication.

Good candidates:

- config helpers
- redaction
- bounded output shaping
- allowlist validation
- reusable DB query safety

Bad candidates:

- large generic tool registries that hide server behavior
- framework-style inheritance layers
- code that forces unrelated servers into the same model

## README Shape

Each new server README should keep the same public-release order:

1. purpose
2. required permissions or scopes
3. install and run
4. environment variables
5. MCP client config example
6. tools and risk levels
7. safe mode behavior
8. optional write mode behavior
9. limits and timeouts
10. known limitations
11. manual verification checklist
