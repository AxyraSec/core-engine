# Security Policy

## Reporting a vulnerability

Please do **not** open a public issue, discussion or pull request.

Email **contact@axyra.fr** with:

- a short description of the issue and its impact,
- steps to reproduce, or a proof of concept,
- the affected version or commit,
- how you would like to be credited (optional).

You can also use GitHub private vulnerability reporting on this repository, if it is enabled.

## What to expect

- We confirm receipt within **3 business days**.
- We send a first assessment within **10 business days**.
- We keep you updated until the issue is fixed.

## Disclosure policy

We follow coordinated disclosure.

- We ask you to keep the issue private until a fix is released, or for **90 days** after your report, whichever comes first.
- We may ask for more time if a fix is complex. We will explain why.
- Once fixed, we publish a GitHub Security Advisory and credit you, unless you prefer to stay anonymous.

We will not take legal action against good-faith research that respects this policy, avoids privacy violations and service disruption, and does not access data beyond what is needed to show the issue.

## Supported versions

The project is in early development. Only the latest release and `main` receive security fixes.

| Version   | Supported |
|-----------|-----------|
| `main`    | Yes       |
| 0.1.x     | Yes       |
| < 0.1.0   | No        |

## Versions in use

Versions the engine is built and checked with. Report issues against these.
This table is generated from `go.mod`, the Dockerfile and the workflows: run `make versions`, do not edit it by hand.

<!-- versions:start -->
| Component | Version |
|---|---|
| Go toolchain | 1.26.8 (minimum `go 1.26.0`) |
| `go.yaml.in/yaml/v3` | v3.0.5 |
| Image `golang:1.26-alpine` | pinned by digest |
| Image `gcr.io/distroless/static-debian13:nonroot` | pinned by digest |
| golangci-lint | v2.14.0 |
| govulncheck | v1.8.0 |
| gitleaks | 8.30.1 |
| trivy | v0.75.0 |
| cosign | v3.1.3 |
| syft | v1.54.0 |
<!-- versions:end -->

GitHub Actions are pinned by full commit SHA in `.github/workflows/`.
Dependency updates are manual (no Dependabot).

## Verifying releases

Release images are signed with cosign (keyless) and ship an SBOM and build provenance.
See [ADR 0004](docs/adr/0004-signed-releases-with-cosign.md).

```sh
cosign verify ghcr.io/axyrasec/core-engine:<tag> \
  --certificate-identity-regexp '^https://github.com/AxyraSec/core-engine/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```
