# Architecture

This document describes TunWarden's intended architecture and current foundation boundaries.

## 1. Architectural principles

TunWarden separates user intent, daemon-owned runtime state, and system networking state.

The CLI is responsible for parsing user intent, rendering output, and managing user-owned local state such as manual profiles. The daemon is responsible for privileged runtime behavior once those behaviors are implemented. Planners build inspectable plans. Executors apply already-validated plans through narrow adapters.

## 2. High-level flow

```text
CLI
  -> profile/subscription user intent
  -> daemon local API
  -> daemon service orchestration
  -> planners
  -> executors/adapters
  -> system state
```

The foundation build intentionally implements only safe subsets of this flow. It does not apply TUN, route, DNS, nftables, firewall, or core process lifecycle changes.

## 3. Boundaries

### 3.1 CLI boundary

The CLI may:

- parse command-line arguments;
- render stable human output and implemented JSON output;
- manage user-owned local intent/state such as profiles;
- call local read-only status, doctor, logs, and recovery helpers in the foundation build;
- call daemon client adapters when daemon-backed behavior exists.

The CLI must not directly mutate privileged system networking state.

### 3.2 Daemon boundary

The daemon owns privileged runtime behavior and active connection state. In v0.1, implemented daemon behavior is intentionally narrow: local Unix socket status and doctor APIs plus systemd startup/journald integration.

Future daemon-owned behavior includes process lifecycle, generated runtime config, active profile snapshots, transactions, and privileged system networking changes.

### 3.3 State boundary

User-owned profiles and subscriptions are persistent user intent. Daemon runtime state and system networking state are separate concerns.

Generated core configs are runtime output, not persistent source of truth.

### 3.4 Current code layout

The current foundation build uses this package layout:

```text
cmd/tunwarden              user-facing CLI entrypoint
cmd/tunwardend             daemon entrypoint
internal/app/cli           CLI command dispatch
internal/app/daemon        daemon process skeleton
internal/api               shared API contracts
internal/client            CLI-side daemon client adapters
internal/daemon            daemon coordination
internal/doctor            safe local diagnostics
internal/engine            core engine lifecycle coordination
internal/logs              read-only journald/system-log integration
internal/network           transaction and network planning model
internal/network/planner   pure network planning logic
internal/network/executor  narrow platform adapters
internal/profile           normalized VPN profile model and user-owned profile storage
internal/recovery          recovery plan and future cleanup behavior
internal/render            CLI output rendering helpers
internal/service           daemon-owned product orchestration
internal/state             runtime and durable state ownership helpers
internal/sub               subscription source model
```

This layout is expected to evolve, but the CLI/daemon boundary and planner/executor split should remain stable architectural constraints.

In the foundation build, `internal/app/cli` may call user-owned profile storage, local read-only diagnostics, read-only system-log inspection, and dry-run recovery planning directly. Privileged or daemon-owned behavior must move behind the daemon client/API boundary once it is implemented.

Package dependency direction is owned by [Package boundaries](./package-boundaries.md).
