# Status command

This document defines the implemented v0.1 behavior for `tunwarden status`.

The command name, arguments, exit codes, stdout/stderr rules, JSON compatibility, and milestone boundaries are owned by [CLI contract](./cli.md). The daemon transport is owned by [Daemon local API](./daemon-api.md). This document owns the current read-only status behavior and its safety boundary.

## Safety boundary

`tunwarden status` is strictly read-only in v0.1.

It may query the local daemon Unix socket. If the daemon is not reachable, it may inspect local TunWarden runtime path metadata as a conservative fallback.

It must not mutate host networking, daemon process lifecycle, core process lifecycle, user profiles, subscriptions, or runtime file contents.

## Human output contract

The default human report starts with:

```text
TunWarden status
```

When the daemon is reachable, v0.1 daemon-backed output has this stable shape:

```text
TunWarden status
Daemon: running
Connection: inactive
Runtime directory: present
Proxy: inactive
TUN: disabled
Stale state: none
```

Clean inactive fallback output on a host without local TunWarden runtime state has this stable shape:

```text
TunWarden status
Daemon: not reachable; using local fallback
Connection: inactive
Runtime directory: missing
Proxy: inactive
TUN: not managed in this build
Stale state: none
```

The daemon-backed v0.1 status reports:

- `Daemon: running` from the daemon process itself;
- `Connection: inactive` because connection lifecycle is not implemented yet;
- `Runtime directory: present` because the daemon owns `/run/tunwarden/` while running;
- `Proxy: inactive` because no proxy process lifecycle exists yet;
- `TUN: disabled` because TUN mode is outside the current milestone.

The local fallback reports:

- daemon unreachable with actionable guidance and a clear statement that local fallback is being used;
- `Connection: inactive` when no stale runtime state is found;
- `Connection: inactive (stale state detected)` when local runtime recovery candidates exist;
- `Connection: unknown (inspection incomplete)` when local runtime state cannot be inspected reliably;
- `Proxy: inactive` because no proxy process lifecycle exists yet;
- `TUN: not managed in this build` because TUN mode is outside the current milestone.

Daemon unavailability alone is not an unhealthy local status when the fallback can prove a clean inactive state. Stale runtime state or incomplete fallback visibility still returns diagnostic exit code `3`.

## v0.1 local runtime inspection

The v0.1 local fallback inspects only documented TunWarden-owned runtime paths:

```text
/run/tunwarden/generated
/run/tunwarden
```

The command does not read generated config contents because generated core configs may contain sensitive material.

Status treats `/run/tunwarden` or `/run/tunwarden/generated` as stale local runtime state and prints recovery guidance:

```text
Guidance: run `tunwarden recover` for the canonical read-only recovery dry-run.
```

If a read-only inspection cannot complete, the command reports incomplete visibility instead of claiming a clean inactive host:

```text
Inspection warnings:
  - could not inspect target: reason
```

Warnings mean the status snapshot had incomplete visibility. Warning-only output must not claim that stale state is absent.

## Redaction

Status output must use the shared TunWarden redaction policy from [State and security requirements](./state-and-security.md).

Default human output must not print full subscription URLs, full share URIs, generated core configs containing credentials, UUID-like user identifiers, passwords, authorization headers, private keys, provider tokens, or secret-looking query parameters.

## Deferred behavior

`status --json` is intentionally not implemented in this PR and currently fails as invalid usage.

Daemon-backed active profile, active mode, proxy listener, core process, and connection health status remain deferred until process lifecycle work exists.
