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

## Enforcement

## Evidence

## Protocol adapters

### MCP

## Module dependencies

## Deployment

### Docker

### On-prem
