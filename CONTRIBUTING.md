# Contributing

## Development Standards

- Keep server implementations in Go.
- Prefer simple packages over framework-style abstractions.
- Add shared code only for genuine cross-cutting concerns.
- Keep stdio servers quiet on stdout except for MCP protocol traffic.
- Route logs to stderr and redact sensitive values.
- Default new capabilities to safe read-only behavior unless there is a strong reason not to.

## Required Checks

Before opening a pull request, run:

```bash
make fmt
make fmt-check
make build
make vet
make test
make lint
make vuln
```

## Pull Request Expectations

- Explain the user-facing capability added or changed.
- Call out security impact explicitly.
- Note any new environment variables, scopes, or external dependencies.
- Add or update tests for safety-sensitive behavior.
- Update the relevant per-server README and `.env.example`.
- Update `CHANGELOG.md` for public-facing changes.
- Keep the tool risk table and manual verification checklist current.

## Code Style

- Use idiomatic Go naming and package layout.
- Prefer explicit error wrapping with context.
- Avoid magic background goroutines and hidden retries.
- Keep tool descriptions clear and operationally honest.

## Public Repo Hygiene

- Never commit real secrets.
- Never add sample configs with live credentials.
- Avoid exaggerated claims about safety or production readiness.
