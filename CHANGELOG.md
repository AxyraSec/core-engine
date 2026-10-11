# Changelog

All notable changes to this project are listed here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project follows [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2026-10-10

First release: an MCP proxy that enforces a policy on every tool call and keeps
tamper-evident evidence of each decision.

### Added

- `engine proxy`: MCP stdio proxy. Every `tools/call` is checked against the policy and
  recorded before it reaches the server; denied calls get JSON-RPC error `-32001`; denied
  tools are hidden from `tools/list`; unparsable messages, batches and calls that cannot be
  recorded are refused ([ADR 0005](https://github.com/AxyraSec/core-engine/blob/main/docs/adr/0005-mcp-stdio-proxy-fails-closed.md)).
- Authorization (`internal/authz`): deny by default, deny wins, exact / `*` / prefix patterns,
  strict YAML policy files, `engine -policy <file>` to check one.
- Evidence (`internal/evidence`): hash-chained decision log stored as JSON Lines, synced on
  every record, verified on open; `engine verify -evidence <file>` to check it.
- Example policies: `policies/example.yaml`, `policies/filesystem-read-only.yaml`.
- `make demo`: end-to-end demo with a fake GitHub MCP server.
- Signed releases: binaries for linux/amd64 and linux/arm64, container image
  `ghcr.io/axyrasec/core-engine`, cosign keyless signatures, SPDX SBOM, build provenance.

[Unreleased]: https://github.com/AxyraSec/core-engine/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/AxyraSec/core-engine/releases/tag/v0.1.0
