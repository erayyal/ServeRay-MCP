# Server README Template

Use this order for every server README in `cmd/<server>-mcp/README.md`.

## Required Sections

1. purpose
2. required permissions or scopes
3. install and run
4. environment variables
5. sample MCP client config
6. tool risk levels
7. safe mode behavior
8. optional write mode behavior
9. limits and timeouts
10. known limitations
11. manual verification checklist

## Risk Level Guidance

- `low`
  listing allowlists, schema inspection, bounded metadata tools
- `medium`
  bounded file reads, remote API reads, browser navigation, raw read-only SQL
- `high`
  any write tool, anything with mutation potential, or any tool gated behind explicit unsafe opt-in

## Public Repo Guidance

- keep claims concrete and operationally honest
- call out what is blocked by default
- document explicit opt-ins
- show the smallest safe config first
- never include real credentials, tokens, cookies, or live URLs with secrets
