## What and why

<!-- What does this change, and why? Link the issue: Fixes #123 -->

## How it was tested

<!-- Commands run, new tests, manual checks. -->

## Security checklist

- [ ] No secrets, tokens or real customer data in code, tests, logs or fixtures.
- [ ] Inputs from agents, tools and MCP servers are validated; failures deny by default.
- [ ] Authorization or enforcement changes are covered by tests, including deny cases.
- [ ] Evidence/audit records are still written for every decision.
- [ ] New dependencies are needed, maintained and pinned (`go.sum`, action SHA, image digest).
- [ ] Workflow changes keep least-privilege `permissions:` and pinned actions.
- [ ] Docs or ADR updated if behavior or architecture changed.
