# TunWarden

TunWarden is a Linux-first VPN client foundation focused on safe TUN networking, predictable DNS behavior, and crash-proof recovery.

The project starts as a CLI + privileged daemon rather than a GUI. The core goal is not to be another configuration wrapper, but to make Linux networking changes explicit, inspectable, reversible, and resilient across Wi-Fi changes, sleep/resume, and process crashes.

## Design goals

- **Linux-first:** Ubuntu is the first target; Debian, Fedora, and Arch should be supported through explicit platform adapters.
- **CLI-first:** GUI can come later. The first-class interface is a deterministic command line.
- **Safe networking:** routes, rules, DNS, nftables, and TUN state must be treated as a transaction.
- **Crash recovery:** every privileged network change must have a cleanup path.
- **Developer-grade diagnostics:** `doctor`, `status`, `explain-routing`, and `panic-reset` should be useful before the first GUI exists.
- **Engine abstraction:** Xray is the first intended protocol engine; AmneziaWG and other engines should be possible later.

## Initial architecture

```text
cmd/tunwarden      user-facing CLI
cmd/tunwardend     privileged daemon
internal/app       application entrypoints
internal/doctor    diagnostics checks
internal/network   route/DNS/TUN transaction model
internal/reset     emergency cleanup plans
internal/profile   normalized VPN profile model
internal/sub       subscription model and future parsers
```

## Current status

This repository is intentionally at foundation stage. The initial code compiles into two binaries and defines the contracts for safe networking, diagnostics, and recovery. It does not yet establish a real VPN tunnel.

## Commands

```bash
go test ./...
go run ./cmd/tunwarden version
go run ./cmd/tunwarden doctor
go run ./cmd/tunwarden panic-reset

go run ./cmd/tunwardend
```

## Safety model

TunWarden should never silently mutate system networking state. The intended lifecycle is:

```text
snapshot -> plan -> apply -> verify -> commit
                         \-> rollback on failure
```

Emergency cleanup is exposed as:

```bash
tunwarden panic-reset
```

By default, this prints the cleanup plan. Destructive execution should remain explicit.

## Documentation

- [Architecture](docs/ARCHITECTURE.md)
- [Networking model](docs/NETWORKING_MODEL.md)
- [Development guide](docs/DEVELOPMENT.md)
- [Roadmap](docs/ROADMAP.md)

## License

A license has not been selected yet. Choose one before accepting external contributions.