# Evidence Chain

## Purpose

Keep a record of every authorization decision that cannot be changed without anyone noticing.
It answers "what was this agent allowed or denied, and when?", and lets an auditor check
that the answer was not rewritten afterwards.

Code: `internal/evidence`.

## What is recorded

One record per decision:

| Field      | Meaning                                                  |
|------------|----------------------------------------------------------|
| `Seq`      | Position in the chain, starting at 1                     |
| `Time`     | When the record was appended, UTC, nanosecond precision  |
| `Subject`  | The agent that made the request                          |
| `Action`   | The requested operation, for example `tools/call`        |
| `Resource` | The target, for example `mcp:github/create_issue`        |
| `Allowed`  | The decision                                             |
| `RuleID`   | The rule that decided, empty if none                     |
| `Reason`   | Stable reason code from `internal/authz`                 |
| `PrevHash` | Hash of the previous record (all zeros for the first)    |
| `Hash`     | SHA-256 of all the fields above                          |

Do not put secrets or tool arguments in these fields: records are meant to be kept and shared with auditors.

## Record format

The hash input is, in this order:

1. the version string `axyra.evidence.v1`,
2. `Seq` as 8 bytes, big-endian,
3. `Time` as RFC 3339 with nanoseconds,
4. `PrevHash` as 32 raw bytes,
5. `Subject`, `Action`, `Resource`,
6. `Allowed` as one byte (`1` or `0`),
7. `RuleID`, `Reason`.

Every string is written as `<length in bytes>:<value>`, so moving characters from one
field to the next changes the hash. The version string lets the format change later.

## Integrity

Each record contains the hash of the previous one. Changing, removing, inserting,
duplicating or reordering any record breaks the chain from that point.

Recomputing the hash of an edited record is not enough: the next record still points
to the old hash. To rewrite history, an attacker has to rewrite every later record.

Limits:

- Removing records **from the end** leaves a valid, shorter chain. To detect it, copy the
  chain head (`Head()`: last `Seq` and `Hash`) somewhere the engine cannot write to.
- Anyone who can write the whole chain can rebuild it from scratch. Signing the head and
  anchoring it outside the engine are planned.

## Storage

In memory only for now. Records are lost on restart. Persistent storage is planned.

## Retention

Not defined yet.

## Verification

`evidence.Verify(records)` walks the chain from the first record and returns the first
broken record, if any (wrong `Seq`, wrong `PrevHash`, or content that does not match `Hash`).

## Export

Not defined yet. `Hash` encodes to lowercase hex in text and JSON.
