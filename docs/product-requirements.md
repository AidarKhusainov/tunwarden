# Product Requirements

## 1. Problem statement

Linux users who rely on Xray-compatible VPN/proxy configurations often experience unstable desktop networking when clients modify TUN interfaces, routes, DNS, firewall rules, or system proxy settings without robust recovery.

Typical failure modes include:

- Wi-Fi appears connected, but applications lose connectivity.
- GNOME/NetworkManager shows a question mark or limited connectivity state.
- DNS stops resolving after disconnecting the client.
- Routes or policy rules remain after the VPN process exits.
- nftables/iptables rules remain after crashes.
- VPN stops after suspend/resume.
- Reconnecting Wi-Fi is required to restore the machine.
- The VPN server route accidentally goes through the VPN TUN interface, causing a traffic loop.

TunWarden exists to make these failure modes explicit, testable, and recoverable.

## 2. Product thesis

TunWarden is a Linux-first, CLI-first VPN/proxy client for Xray-compatible configurations.

The project should prioritize:

1. reliable networking,
2. safe rollback,
3. clear diagnostics,
4. subscription/profile interoperability,
5. minimal but powerful UX for technical users.

The project should not initially prioritize:

1. GUI,
2. visual profile management,
3. mobile support,
4. broad protocol coverage at the expense of reliability,
5. hiding networking details from advanced users.

## 3. Target users

### Primary users

- Linux desktop users on Ubuntu/Debian.
- Backend developers, DevOps engineers, sysadmins, and technical power users.
- Users who understand CLI tools and want deterministic behavior.
- Users who import subscriptions from panels such as Remnawave, 3x-ui, and similar Xray-compatible systems.

### Secondary users

- Linux users who want a future GUI but can start with CLI.
- Users who need stable VPN behavior across laptop suspend/resume.
- Users who want a safe alternative to ad-hoc scripts.

## 4. Product goals

### G1. CLI-first operation

TunWarden must be fully usable through CLI commands.

Required commands:

```bash
tunwarden status
tunwarden connect <profile-or-node>
tunwarden disconnect
tunwarden doctor
tunwarden logs
tunwarden panic-reset
```

### G2. Safe privileged networking

The project must never rely on a privileged GUI process or SUID frontend for normal operation.

Required model:

```text
unprivileged CLI/user client
  -> Unix socket or D-Bus
privileged root daemon
  -> TUN, routes, DNS, nftables, core lifecycle
```

### G3. Transactional network changes

Every VPN activation must be treated as a transaction:

```text
plan -> snapshot -> apply -> verify -> commit
                         ↓
                      rollback
```

TunWarden must keep enough state to clean up after failed attempts and crashes.

### G4. Reliable laptop behavior

TunWarden must handle:

- suspend/resume,
- Wi-Fi reconnect,
- default interface change,
- DHCP renewal,
- DNS change,
- NetworkManager connectivity changes,
- core process crash.

### G5. Observable diagnostics

TunWarden must explain its decisions.

Examples:

```bash
tunwarden plan my-profile
tunwarden doctor
tunwarden explain routes
tunwarden explain dns
tunwarden explain firewall
```

## 5. MVP scope

### Included in MVP

- Linux only.
- CLI only.
- Root daemon managed by systemd.
- Xray core lifecycle management.
- Manual node import for VLESS, VMess, Trojan, Shadowsocks where feasible.
- Base64 subscription import.
- TUN full-tunnel mode.
- Proxy-only mode.
- systemd-resolved DNS backend.
- NetworkManager integration.
- nftables-based firewall/kill-switch foundation.
- `doctor` diagnostics.
- `panic-reset` cleanup.

### Excluded from MVP

- GUI.
- Windows/macOS/mobile.
- Browser extension.
- Router mode.
- Automatic core updater.
- Advanced visual routing editor.
- Enterprise device management.
- Full AmneziaWG support.

## 6. Functional requirements

### FR-001: Profile management

TunWarden must support storing, listing, showing, and deleting profiles.

A profile must include:

- profile ID,
- display name,
- source type,
- protocol,
- server address,
- server port,
- security settings,
- transport settings,
- DNS policy,
- routing policy,
- metadata.

### FR-002: Subscription management

TunWarden must support adding and updating subscription sources.

A subscription source must include:

- source ID,
- URL,
- format detection mode,
- update interval,
- last update status,
- imported profiles/nodes,
- optional provider metadata.

### FR-003: Connection lifecycle

TunWarden must support:

- connect,
- disconnect,
- reconnect,
- status,
- graceful shutdown,
- forced cleanup.

### FR-004: Dry-run planning

TunWarden must support a planning mode that prints intended changes without applying them.

Example output categories:

- TUN interface changes,
- routes,
- policy rules,
- DNS changes,
- nftables changes,
- core process config,
- rollback state location.

### FR-005: Diagnostics

`tunwarden doctor` must check:

- daemon status,
- core process status,
- TUN interface status,
- default route,
- policy rules,
- route to VPN server,
- DNS configuration,
- DNS resolution,
- nftables state,
- NetworkManager state,
- external TCP connectivity,
- optional UDP connectivity,
- stale TunWarden-owned state.

### FR-006: Panic reset

`tunwarden panic-reset` must remove TunWarden-owned volatile state even if the daemon has crashed.

It must clean:

- TunWarden TUN interfaces,
- TunWarden policy rules,
- TunWarden routing tables,
- TunWarden nftables tables/chains,
- TunWarden DNS settings where possible,
- TunWarden core processes,
- pending transaction state.

## 7. Non-functional requirements

### NFR-001: Safety

The project must prefer restoring direct connectivity over preserving a broken VPN tunnel, unless the user enabled strict kill-switch mode.

### NFR-002: Auditability

Every privileged change must be logged with enough detail to debug failures.

### NFR-003: Idempotency

Cleanup operations must be safe to run multiple times.

### NFR-004: Minimal assumptions

TunWarden must not assume:

- interface names are stable,
- Wi-Fi is the only uplink,
- IPv6 is available,
- systemd-resolved is always configured the same way,
- NetworkManager connectivity state is always accurate,
- subscription format is always correctly declared.

### NFR-005: Testability

Core planners must be testable without root privileges by producing desired state plans from input snapshots.

## 8. Success metrics

Early success should be measured by reliability, not feature count.

Examples:

- Connection/disconnection leaves no stale TunWarden routes/rules/firewall state.
- Suspend/resume reconnects automatically on Ubuntu LTS.
- `panic-reset` restores direct internet connectivity in common failure cases.
- `doctor` reports actionable causes for DNS/routing failures.
- At least one common subscription source can be imported end-to-end.
