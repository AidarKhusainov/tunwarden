# CLI contract

This document defines the canonical TunWarden command-line interface, safety semantics, output expectations, and milestone boundaries.

## General rules

- Commands use user-task language.
- Read-only commands must not mutate host networking.
- High-impact flags such as `--execute` and `--yes` stay long-only.
- Errors go to stderr and stable command output goes to stdout when the implementation supports that separation.
- JSON output uses `schema_version` once implemented for a command.
- Human output and JSON output must share the same redaction policy.

## Commands

### Doctor

```bash
tunwarden doctor [--json]
tunwarden doctor --core --xray <path> [--json]
tunwarden doctor --network [--json]
tunwarden doctor --dns [--json]
tunwarden doctor --routes [--json]
tunwarden doctor --firewall [--json]
```

Purpose: explain environment and runtime health.

Mutation level: read-only.

Daemon requirement: optional. The default command must use daemon-backed diagnostics when available and local read-only diagnostics otherwise. The v0.1 `doctor --core --xray <path>` scope is explicitly local-only.

Implemented foundation doctor behavior:

- default human output with daemon-backed diagnostics or local fallback;
- local host diagnostics for platform, command availability, default route, default interface, and stale TunWarden-owned resources;
- local-only `doctor --core --xray <path>` validation of an explicitly provided Xray binary;
- `doctor --core --xray <path> --json` with the common top-level JSON shape and `checks`;
- shared human/JSON redaction for doctor output.

`doctor --json` without `--core` is deferred to a separate issue. Until implemented, it must fail fast as invalid usage with exit code `2`.

`doctor --core` without `--xray <path>` is deferred in v0.1. It must fail fast as invalid usage with exit code `2` instead of guessing a default Xray path.

`doctor --core` is the preferred public UX for validating the Xray binary and runtime core health. A lower-level `core check` command is not part of the v0.1 public contract.

### Logs

```bash
tunwarden logs [--follow] [--daemon] [--core] [--since <duration>]
tunwarden logs -f
```

Purpose: inspect TunWarden daemon and core logs.

Mutation level: read-only.

Implemented v0.1 journald-backed log behavior:

- human output only;
- recent `tunwardend.service` logs through the system journal with `journalctl --system`;
- `--follow` and `-f` for live log following;
- `--daemon` as the explicit daemon log source;
- `--core` for Xray lifecycle and forwarded stdout/stderr lines marked by the daemon;
- `--since <duration>` and `--since=<duration>` passed to journalctl, including relative values such as `-1h`;
- shared human-output redaction for each printed log line;
- clear no-core-log guidance when `--core` finds no recent matching lines in non-follow mode.

`logs --json` is deferred to a separate issue. Until implemented, it must fail fast as invalid usage with exit code `2`.

If `journalctl` is unavailable, the command must fail clearly with an actionable message. If the current user cannot read the system journal, the command must surface the redacted `journalctl` error.

`-f` may alias `--follow` because it is a common log-following pattern.
