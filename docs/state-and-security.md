# State and Security Requirements

This document owns podlaz state layout, output redaction, daemon hardening, recovery safety boundaries, and core process safety rules.

## 1. State ownership model

podlaz must keep three levels of state separate.

### 1.1 User intent and user state

User intent is owned by the user-facing CLI experience.

Examples:

- imported profiles,
- subscription sources,
- user preferences,
- selected defaults,
- import metadata useful to the user.

Preferred locations:

```text
User config:
  $XDG_CONFIG_HOME/podlaz/
  default: ~/.config/podlaz/

User state:
  $XDG_STATE_HOME/podlaz/
  default: ~/.local/state/podlaz/

User cache:
  $XDG_CACHE_HOME/podlaz/
  default: ~/.cache/podlaz/
```

Rules:

- User files must not require root ownership.
- Profile and subscription source of truth should not be hidden only in daemon-private directories.
- The daemon should receive the selected intent through its local API instead of reading arbitrary user home directories.
- If the daemon later owns shared system-wide profiles, that must be an explicit separate feature.

### 1.2 Daemon runtime and daemon state

Daemon state is owned by `podlazd`.

Examples:

- active connection state,
- active profile snapshot,
- lock files,
- generated runtime config,
- child process state,
- pending or committed transaction state,
- daemon socket.

Preferred locations:

```text
Runtime:
  /run/podlaz/

Persistent daemon state:
  /var/lib/podlaz/

Daemon logs:
  journald first, package logs only if needed later
```

For packaged systemd units, prefer:

```ini
RuntimeDirectory=podlaz
StateDirectory=podlaz
```

`LogsDirectory=podlaz` is intentionally not required while the daemon logs to stdout/stderr and the unit sends those streams to journald. Add a logs directory only when file-based package logs become a real product requirement.

#### Transaction runtime state

The implemented transaction state schema is volatile daemon runtime state and is stored under:

```text
/run/podlaz/transactions/<id>.json
```

Transaction files must be:

- owned by podlaz and include `owner: "podlaz"`;
- versioned with `schema_version: "podlaz.transaction.v1"`;
- written with file mode `0600` under a daemon-owned runtime directory;
- written atomically with temp-file fsync, rename, and directory fsync;
- safe to scan repeatedly after daemon restart;
- free of persistent secrets.

Transaction files may store non-secret rollback metadata for podlaz-owned TUN devices, routes, policy rules, DNS state, nftables tables/chains, generated runtime config paths, and child-process labels or PIDs. They must not store share URIs, subscription URLs with provider tokens, passwords, authorization headers, private keys, or provider API tokens.

`status`, `doctor`, and `recover` must be able to explain pending, failed, rolling-back, or otherwise stale transaction state using only redacted summaries. They must not apply cleanup as part of read-only inspection.

#### Recovery cleanup boundary

Recovery has two distinct modes:

- `podlaz recover` is a read-only scanner and must not mutate host state.
- `podlaz recover --execute --yes` is an explicit cleanup intent sent by the CLI to `podlazd`; the CLI must not perform privileged host cleanup directly.

Daemon-owned recovery execution may mutate only clearly podlaz-owned volatile state. Eligible targets are limited to documented runtime children, generated runtime configs, the managed TUN interface, podlaz-owned nftables objects, and route, rule, DNS, TUN, and generated-config rollback entries recorded as podlaz-owned transaction metadata.

Ambiguous rollback metadata must be reported as skipped and left unchanged. A stale PID recorded in transaction metadata is not sufficient process identity; recovery must not signal a process from PID metadata alone unless a future daemon-supervised identity check can prove the process is still the podlaz-owned child.

The runtime root `/run/podlaz` must not be removed wholesale. Recovery may remove specific stale owned children only when their paths are proven to be inside the documented podlaz runtime layout.

Transaction state should be removed only after the cleanup sequence completes safely. If any cleanup action fails or any ambiguous resource is skipped, the transaction file should remain available for later diagnostics or a future safer recovery attempt.

### 1.3 System networking state

System networking state is not persistent application data.

Examples:

- TUN interface,
- routes,
- policy rules,
- DNS link configuration,
- nftables tables and chains.

Rules:

- It must be applied only through daemon-owned transactions.
- It must be identifiable as podlaz-owned.
- It must be inspectable through `plan`, `status`, `doctor`, and `recover`.
- It must be recoverable without relying on the original CLI process.

## 2. JSON compatibility

JSON output is a public interface starting with v0.1.

Rules:

- Every JSON response must include `schema_version`.
- Existing field names and meanings must not change without a documented compatibility note.
- New fields may be added.
- Consumers must ignore unknown fields.
- Human output and JSON output must apply the same redaction policy.

Common top-level fields:

```json
{
  "schema_version": "v1",
  "status": "ok|warn|fail",
  "warnings": [],
  "errors": []
}
```

Command-specific top-level fields:

```text
status:
  daemon
  connection
  runtime
  transactions

doctor:
  checks

plan:
  mode
  plan
  steps
  rollback_steps

recover:
  mode
  recovery
```

Daemon-backed status may include `transactions`. Each item is a redacted summary with stable facts only:

```json
{
  "id": "tx-1",
  "state": "applying",
  "rollback_available": true,
  "requires_cleanup": true,
  "path": "/run/podlaz/transactions/tx-1.json"
}
```

Human-readable transaction phrases such as `pending apply` are rendered by clients from `state` and cleanup flags; they are not a separate source of truth in the daemon API.

Recovery execute JSON must use `status: "fail"` when cleanup actions fail. It must use `status: "warn"` when recovery is incomplete, for example when transaction state is preserved because ambiguous resources were skipped. A warning status is not a successful full cleanup for automation.

## 3. Output redaction

podlaz must be observable without leaking sensitive material.

Default human output and `--json` output must redact:

- full subscription URLs,
- full share URIs,
- UUID-like user identifiers,
- passwords and private keys,
- authorization headers,
- generated core config content that includes credentials,
- provider tokens and query parameters that look secret.

Allowed output shape examples:

```text
uuid: abcd…7890
subscription: https://example.com/sub?token=REDACTED
```

Rules:

- Generated core configs must not be logged in full by default.
- Transaction files must reject persistent secret-looking keys and values before they are written.
- `logs`, `doctor`, `status`, `plan`, and `recover` must use the same redaction helpers.
- A future explicit debug mode must document exactly what additional data it reveals.

## 4. Confirmation model

Commands that remove user state or execute recovery cleanup must have consistent confirmation behavior.

Rules:

- In an interactive TTY, ask for confirmation unless `--yes` is passed.
- In non-interactive mode, fail unless `--yes` is passed.
- In `--json` mode, fail unless `--yes` is passed.
- High-impact flags such as `--execute` and `--yes` must not have short aliases.

Examples:

```bash
podlaz profile delete <profile-id> --yes
podlaz subscription delete <subscription-id> --yes
podlaz recover --execute --yes
```

`disconnect` is a normal lifecycle command and should not require confirmation unless a future flag changes its meaning beyond stopping the active connection.

## 5. systemd service hardening

The daemon service must start from least privilege. Every relaxation must be justified in documentation or in comments near the unit file.

Packaged baseline service behavior:

- `packaging/sysusers.d/podlaz.conf` creates the dedicated unprivileged `podlaz` daemon access group and the dedicated `podlaz-xray` proxy-core child identity.
- `packaging/systemd/podlazd.service` starts `podlazd` as `root:podlaz` so the daemon can own TUN, route, policy-rule, DNS, nftables, and child-lifecycle operations while keeping the CLI unprivileged.
- Xray child processes must be started under the dedicated `podlaz-xray:podlaz-xray` execution identity instead of inheriting UID 0.
- Generated Xray runtime config is owned by `root:podlaz-xray`, the generated config directory is mode `0750`, and the generated config file is mode `0640`. This is the documented private equivalent to `0600`: only the daemon/root owner and the dedicated Xray child group can read the generated config.
- The packaged unit sets `UMask=0077`; daemon runtime files are private by default, and the daemon-created control socket is explicitly opened to mode `0660`.
- The dedicated `podlaz` group is the packaged socket access boundary for CLI commands that use the daemon.
- `RuntimeDirectory=podlaz` with `RuntimeDirectoryMode=0711` lets local users traverse `/run/podlaz` to reach the daemon socket path, but does not let them list daemon-private runtime state.
- Only `/run/podlaz/podlazd.sock` is intentionally exposed to `podlaz` group members for packaged CLI access.
- Generated configs under `/run/podlaz/generated`, transaction files under `/run/podlaz/transactions`, lock files, and other daemon runtime children remain daemon-private unless a later contract explicitly changes them.
- The daemon itself applies socket mode `0660` to `/run/podlaz/podlazd.sock`.
- `StateDirectory=podlaz` with `StateDirectoryMode=0700` reserves `/var/lib/podlaz` for daemon-owned persistent state without exposing it to the CLI access group.
- `StandardOutput=journal` and `StandardError=journal` make daemon logs visible through `journalctl -u podlazd`.

Current hardening baseline:

```ini
User=root
Group=podlaz
UMask=0077
NoNewPrivileges=yes
CapabilityBoundingSet=CAP_CHOWN CAP_SETUID CAP_SETGID CAP_KILL CAP_NET_ADMIN
AmbientCapabilities=CAP_SETUID CAP_KILL
ProtectSystem=strict
ProtectHome=yes
PrivateTmp=yes
ProtectControlGroups=yes
RestrictSUIDSGID=yes
LockPersonality=yes
MemoryDenyWriteExecute=yes
RuntimeDirectory=podlaz
RuntimeDirectoryMode=0711
StateDirectory=podlaz
StateDirectoryMode=0700
```

Privilege status for the current milestone:

- The packaged daemon is privileged because full-tunnel mode and daemon-owned recovery mutate Linux networking state; the CLI must remain an unprivileged client that sends intents through the daemon socket only.
- `CAP_NET_ADMIN` is kept in the service bounding set for TUN devices, routes, policy rules, DNS link state, and nftables/firewall state, but it is not ambient. It must not be inherited by Xray.
- `CAP_SETUID` and `CAP_SETGID` allow the daemon to drop the Xray child into the dedicated `podlaz-xray:podlaz-xray` identity and clear supplementary groups.
- `CAP_KILL` allows the daemon to stop only the child processes it supervises and records; recovery still must not signal stale PIDs from metadata alone.
- `CAP_CHOWN` allows the daemon to write runtime config as `root:podlaz-xray` with group-readable private file mode for the child.
- `NoNewPrivileges=yes`, `RestrictSUIDSGID=yes`, `MemoryDenyWriteExecute=yes`, `ProtectSystem=strict`, `ProtectHome=yes`, `PrivateTmp=yes`, `ProtectControlGroups=yes`, and `LockPersonality=yes` remain part of the packaged hardening baseline.
- Packaged proxy-only mode must not grant `CAP_NET_ADMIN`, `CAP_NET_RAW`, broad file capabilities, or ambient capabilities to Xray.
- Add `CAP_NET_RAW` only if a concrete health check or networking feature needs it and the PR documents why.
- Broad file permission bypass capabilities must not be in the baseline.
- `PrivateDevices=yes`, restrictive address-family filters, and kernel-tunable protections are deferred because they can conflict with `/dev/net/tun`, netlink, routing, or nftables work and must be validated together with those features.
- Any future packaged service privilege expansion must update this document and the unit file in the same change, including the exact capability set and why each capability is required.

## 6. Core engine process safety

The core engine process is a child process managed by the daemon, not the owner of podlaz system state.

Rules:

- The core process must not inherit broad daemon privileges unless strictly required.
- In packaged mode, `podlazd` runs as `root:podlaz`, but Xray must run as the unprivileged `podlaz-xray:podlaz-xray` child identity.
- The proxy-only Xray child must not inherit supplementary groups.
- Xray must not inherit `CAP_NET_ADMIN`, `CAP_NET_RAW`, `CAP_SETUID`, `CAP_SETGID`, `CAP_KILL`, or broad file capabilities.
- Generated core configs must be private as `root:podlaz-xray` with directory mode `0750` and file mode `0640`.
- Generated core configs must be written atomically.
- Generated core configs must be treated as runtime output, not persistent source of truth.
- Generated core configs must not be printed or logged in full by default.
- The daemon must be able to stop the core process and explain core process failures through `status`, `doctor`, and `logs`.
