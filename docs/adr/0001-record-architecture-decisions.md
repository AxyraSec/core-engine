# 1. Record architecture decisions

- Status: Accepted
- Date: 2026-10-04

## Context

We make design choices that shape the engine for years. If we do not write them
down, we forget why we made them, and new contributors have to guess.

## Decision

We record important architecture decisions as short Markdown files in `docs/adr/`,
using the format described by Michael Nygard.

- One decision per file, named `NNNN-short-title.md`, numbered in order.
- Each ADR has: Status, Date, Context, Decision, Consequences.
- Status is one of: Proposed, Accepted, Deprecated, Superseded by ADR-NNNN.
- An accepted ADR is not rewritten. To change a decision, write a new ADR that supersedes it.
- ADRs are added through a normal pull request.

## Consequences

- The reasons behind a decision stay next to the code.
- Writing an ADR takes a few minutes per decision. That cost is small.
- Reviewers can push back on a decision before it lands.
