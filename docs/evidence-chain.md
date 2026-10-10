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

`evidence.FileLog` stores the chain in a JSON Lines file, one record per line:

```json
{"seq":2,"time":"2026-10-10T19:00:44.63666Z","subject":"agent:billing-bot","action":"tools/call","resource":"mcp:github/delete_repo","allowed":false,"rule_id":"deny-github-delete-repo","reason":"denied_by_rule","prev_hash":"8c1f…","hash":"3e9a…"}
```

- The file is created with mode `0600`. Records are only appended.
- Each `Append` writes the line and syncs the file before it returns. If it fails, the
  MCP proxy denies the call.
- On open, the whole file is verified. A broken file, a partly written last line or an
  unknown field makes the open fail: the engine will not extend a chain it cannot trust.
- After a restart, new records continue the chain found in the file.
- Only one process should write a given file.

`evidence.Chain` is the same chain kept in memory, for tests and tools.

## Retention

Not defined yet. The file grows without limit.

## Verification

```sh
engine verify -evidence evidence.jsonl
# evidence OK: 3 records, head 9fabc484…
```

It reads the file from the first record and reports the first broken record, if any
(wrong `seq`, wrong `prev_hash`, content that does not match `hash`, or a line that is
not a valid record). In Go: `evidence.VerifyFile(path)` or `evidence.Verify(records)`.

Keep a copy of the printed head (seq + hash) outside the machine to detect truncation.

## Export

The JSON Lines file is the export format. `hash` and `prev_hash` are lowercase hex.
