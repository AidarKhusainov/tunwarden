# Logs command

`tunwarden logs` is the implemented v0.1 read-only command for inspecting TunWarden daemon logs from journald.

Canonical CLI shape is owned by [CLI contract](./cli.md). This document describes the implemented behavior.

## Command shape

```bash
tunwarden logs
tunwarden logs --follow
tunwarden logs -f
tunwarden logs --daemon
tunwarden logs --since "1 hour ago"
```

## Behavior

The command prints a stable header and then streams redacted `journalctl` output for the `tunwardend.service` systemd unit:

```text
TunWarden daemon logs
Jun 03 12:00:00 host tunwardend[1234]: tunwardend: daemon started
Jun 03 12:00:01 host tunwardend[1234]: tunwardend: status request handled
```

Default mode shows recent daemon logs using a bounded `journalctl --lines` query. `--follow` and `-f` pass through to `journalctl --follow` for live inspection. `--daemon` is accepted as the explicit daemon-log source and is currently equivalent to the default source.

`--since <duration>` is passed to `journalctl --since <duration>` and can use journalctl-compatible values such as `1 hour ago`.

## Safety boundary

`logs` is read-only. It must not mutate daemon state, host networking, routes, DNS, nftables, firewall state, runtime files, or user configuration.

## Redaction

Every log line printed by `tunwarden logs` goes through the shared TunWarden human-output redaction helper before it reaches stdout. This keeps logs aligned with the documented redaction policy for status, doctor, logs, plan, and recover output.

Daemon lifecycle and local API request logs intentionally avoid request/response payloads and generated runtime configuration content.

## Failure behavior

If `journalctl` is missing, the command fails with an actionable error explaining that systemd journal tools or a systemd/journald host are required.

If `journalctl` exits non-zero, the command returns a runtime error with redacted journalctl stderr.

## Deferred behavior

The following are not implemented in v0.1:

- `tunwarden logs --json`
- `tunwarden logs --core`
- Xray/core logs
- file-based log fallback
- log rotation management
- metrics or tracing
