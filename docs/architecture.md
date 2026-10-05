# Architecture

## Overview

## Request flow

## Identity

## Delegation

## Authorization

`internal/authz` decides if a request is allowed. A request has a subject (the agent),
an action (for example `tools/call`) and a resource (for example `mcp:github/create_issue`).

- Deny by default: a request is allowed only if an allow rule matches it.
- A matching deny rule always wins over allow rules.
- A request with an empty field is denied.
- Patterns are exact values, `*`, or a prefix ending with `*` (`mcp:github/*`).
- Rules are validated when the policy is built. A built policy cannot change.

Each decision carries the rule ID and a stable reason, for the evidence log.

### Policy file

Policies are written in YAML. See [policies/example.yaml](../policies/example.yaml).

```yaml
version: 1
rules:
  - id: allow-billing-bot-github
    effect: allow            # allow or deny
    subjects: ["agent:billing-bot"]
    actions: ["tools/call"]
    resources: ["mcp:github/*"]
```

Loading is strict, and a file is either fully loaded or rejected:

- `version: 1` is required.
- Unknown fields (typos such as `efect`) and duplicate keys are rejected.
- One YAML document per file, at most 1 MiB.
- Every rule then goes through the same validation as rules built in code.

`engine -policy <file>` checks a file and exits with a non-zero code if it is invalid.

## Enforcement

## Evidence

`internal/evidence` records every decision in a hash chain: each record holds the hash of
the previous one, so any later change is detected by `Verify`. It does not import any other
internal module; callers copy the decision into an `evidence.Entry`.
See [evidence-chain.md](evidence-chain.md).

## Protocol adapters

### MCP

## Module dependencies

## Deployment

### Docker

### On-prem
