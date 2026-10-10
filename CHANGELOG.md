# Changelog

All notable changes to this project are listed here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- Repository structure, Go module and minimal `engine` binary.
- Project docs: README, contributing guide, security policy, ADRs.
- `internal/authz`: deny-by-default policy evaluation (allow/deny rules, prefix wildcards, deny wins).
- `internal/evidence`: in-memory, hash-chained evidence log with `Verify` to detect tampering.
- YAML policy files (`authz.LoadPolicyFile`), strict loading, and `engine -policy <file>` to check one.
- `engine proxy`: MCP stdio proxy that enforces the policy on every `tools/call`, records
  each decision, hides denied tools from `tools/list`, and fails closed (ADR 0005).
- `evidence.FileLog`: evidence chain persisted as JSON Lines, synced on every record,
  verified on open; `engine verify -evidence <file>` to check a log.
- `make demo`: end-to-end demo with a fake GitHub MCP server.
