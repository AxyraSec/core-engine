# 2. Use Go as the implementation language

- Status: Accepted
- Date: 2026-10-04

## Context

The engine sits in the path of every agent action. It must be fast, predictable,
safe with concurrency, and easy to ship on-prem and in containers. It is also a
security product, so the dependency tree and the build must be easy to audit.

## Decision

We write the engine in Go, using the latest stable release (Go 1.26 at the time of writing).

- Builds produce a single static binary (`CGO_ENABLED=0`).
- We prefer the standard library. Each new dependency must be justified in the PR.
- We use the Go toolchain for security checks: `govulncheck`, race detector, `go vet`.

## Consequences

- Simple deployment: one binary, small container images (distroless or scratch).
- Good fit for network services and concurrency.
- Strong supply-chain tooling: module checksums, `govulncheck`, reproducible builds.
- Go has no sum types or rich generics. Policy logic may need more code than in some other languages.
- We follow the Go release cycle and upgrade the `go` directive when a new stable version ships.
