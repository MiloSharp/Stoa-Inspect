# Stoa-Inspect

Stoa-Inspect is a read-only Go command-line tool for auditing event streams exported by [The Stoa](https://github.com/MiloSharp/Stoa). It produces a concise, human-readable operations report by default and can emit the same findings as structured JSON for automation.

The tool never connects to or modifies a running Stoa instance. It reads an export, validates it, reports what it found, and exits.

## What it checks

- Missing, duplicate, or out-of-order event sequences
- Duplicate, missing, or malformed event identifiers
- Simulated-time regressions
- Required event fields and visibility values
- Activity scheduling, completion, interruption, resumption, and cancellation transitions
- Concurrent activity resource conflicts
- Unknown activity and proposal references
- Proposal lifecycle and resolution values
- Duplicate pause or invalid resume events
- Basic invariants for time advances, world updates, scenarios, and private memories

## Build

Go 1.24 or newer is required. The project uses only the Go standard library.

```powershell
go test ./...
go build -o bin/stoa-inspect.exe ./cmd/stoa-inspect
```

## Human-readable audit

```powershell
go run ./cmd/stoa-inspect audit examples/valid.ndjson
```

Example result:

```text
STOA EVENT AUDIT
Source:       examples/valid.ndjson
Result:       PASS
Events:       6
Sequence:     1-6
Final clock:  t=4
Findings:     0 errors, 0 warnings, 0 information

FINDINGS
No findings match this view.
```

An inconsistent stream produces actionable findings and exits with status 1:

```powershell
go run ./cmd/stoa-inspect audit examples/invalid.ndjson
```

## JSON audit

```powershell
go run ./cmd/stoa-inspect audit --format json examples/valid.ndjson
```

JSON output uses a versioned schema and contains the audit summary, findings, filters, and per-actor lifecycle totals.

## Filtering and output

```powershell
stoa-inspect audit --severity warning events.ndjson
stoa-inspect audit --actor pallas events.ndjson
stoa-inspect audit --from-sequence 800 --to-sequence 900 events.ndjson
stoa-inspect audit --format json --output audit.json events.ndjson
Get-Content events.ndjson | stoa-inspect audit -
```

Actor filtering retains global integrity findings such as sequence gaps, because those affect the whole stream.

## Accepted input

Stoa-Inspect accepts a JSON event array, a Stoa state object containing an `events` array, NDJSON, or PostgreSQL-style rows using `event_id`, `event_type`, `simulated_at`, and a nested `payload`. Input is limited to 64 MiB per run as a defensive boundary.

## Exit codes

- `0` — audit completed without errors; warnings may be present
- `1` — the audit found one or more integrity errors
- `2` — invalid arguments, unreadable input, or an output failure

## Design

Parsing, validation, and rendering are separate packages. Both output formats consume the same audit result, so the human and JSON reports cannot disagree about the underlying findings.
