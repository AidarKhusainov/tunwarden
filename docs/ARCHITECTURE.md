# Architecture

TunWarden is designed as a Linux-first CLI and daemon system.

## Components

```text
User shell
  -> tunwarden CLI
      -> unprivileged UX, dry-runs, diagnostics, profile management

System service
  -> tunwardend daemon
      -> privileged network transactions, TUN lifecycle, DNS, routes, firewall

Protocol engines
  -> Xray first
  -> AmneziaWG later
```

## Why CLI + daemon

VPN clients often break Linux networking because UI processes directly mutate routes, DNS, or firewall state and then crash or exit without cleanup. TunWarden keeps privileged mutations inside one supervised daemon so that recovery can be centralized and tested.

The CLI should remain usable without root for read-only operations. Privileged operations should cross a narrow IPC boundary later.

## Core rule

Networking changes are not side effects. They are transactions:

```text
snapshot -> plan -> apply -> verify -> commit
                         \-> rollback
```

## Packages

- `cmd/tunwarden`: user-facing command line.
- `cmd/tunwardend`: privileged daemon process.
- `internal/app`: executable entrypoints and command dispatch.
- `internal/doctor`: safe diagnostics.
- `internal/network`: transaction and network planning model.
- `internal/profile`: normalized profile model.
- `internal/reset`: emergency recovery plan.
- `internal/sub`: subscription source model.

## Non-goals for the foundation stage

- No GUI.
- No SUID helper.
- No direct networking mutation before snapshot/rollback is implemented.
- No full subscription parser before the normalized profile model is stable.
