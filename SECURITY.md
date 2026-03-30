# Security Policy

## Scope

Security reports are welcome for:

- credential handling
- secret redaction failures
- filesystem escape paths
- SQL safety bypasses
- unsafe remote API behavior
- transport or JSON-RPC correctness issues
- panic recovery failures that leak sensitive information
- outbound HTTP SSRF bypasses
- release packaging issues that ship incorrect binaries or checksums

## Reporting a Vulnerability

Do not open a public issue for an active vulnerability.

Instead, report it privately with:

- affected server or package
- reproduction steps
- expected impact
- configuration assumptions
- any proof-of-concept payload that demonstrates the issue safely

If you fork this repository publicly, add your own private reporting channel before distribution.

## Supported Versions

Public support should normally track:

- the latest tagged release
- the current default branch before the next tag

## Response Expectations

- Triage target: within 5 business days
- Initial remediation assessment: as soon as reproduction is confirmed
- Coordinated disclosure preferred for user-impacting issues

## Hardening Assumptions

This repository assumes:

- users control their own credentials and scopes
- secrets are injected at runtime, not committed
- deployments use least-privileged backend identities
- production systems are isolated appropriately for the operator’s environment

If you discover a gap between those assumptions and the actual implementation, report it.
