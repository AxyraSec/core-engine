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

The project is in early development. Only the latest release receives security fixes.

| Version         | Supported |
|-----------------|-----------|
| latest release  | Yes       |
| older releases  | No        |

## Verifying releases

Release images are signed with cosign (keyless). See [ADR 0004](docs/adr/0004-signed-releases-with-cosign.md).
