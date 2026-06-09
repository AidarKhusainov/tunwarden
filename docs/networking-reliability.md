# Networking and Reliability Requirements

## 1. Purpose

This document defines the networking invariants TunWarden must preserve.

The main requirement is simple:

> TunWarden must not leave the user's Linux machine without recoverable networking.

VPN correctness is not enough. Cleanup, rollback, recovery, and diagnostics are part of the product.

## 2. Supported initial environment

Initial target:

- Ubuntu LTS desktop,
- Debian stable desktop,
- NetworkManager,
- systemd,
- systemd-resolved,
- nftables,
- iproute2,
- Linux TUN device.

The implementation must be structured so Fedora and Arch can be added without rewriting core logic.

## 3. Networking modes

### 3.1 Proxy-only mode

Proxy-only mode must start first because it is the safest mode.

Expected behavior:

- no default route changes,
- no TUN interface,
- no nftables redirect,
- local SOCKS/HTTP/mixed inbound if supported,
- useful for validating profiles and subscriptions.

### 3.2 TUN full-tunnel mode

TUN mode is the primary VPN-like mode.

Expected behavior:

- create a stable TUN interface, for example `tunwarden0`,
- route general traffic through the TUN interface,
- route the VPN server itself outside the TUN interface,
- configure DNS intentionally,
- prevent traffic loops,
- clean up on disconnect/crash.

The current executor layer has daemon-owned transaction execution for the TUN interface, routes, policy rules, systemd-resolved per-link DNS, and TunWarden-owned nftables state. User-visible `connect --mode tun` must still be gated on real TUN-mode Xray runtime config generation and basic connectivity verification. It must not start proxy-only Xray config and report it as an active TUN connection.

### 3.3 Split-tunnel mode

Split-tunnel mode is future scope.

It must be implemented only after full-tunnel mode has strong diagnostics and rollback.

## 4. Required invariants

### INV-001: Server route must bypass VPN TUN

Traffic to the active proxy/VPN server must not be routed through `tunwarden0`.

A health check must verify:

```bash
ip route get <server-ip>
```

The result must use the physical/default uplink, not the TunWarden TUN interface.

For hostname profile servers, read-only snapshot collection must resolve the hostname under a bounded timeout before performing server-route lookup. DNS resolution failure must be reported as incomplete visibility instead of being hidden by planner defaults.

### INV-002: TunWarden-owned state must be identifiable

Routes, rules, nftables tables/chains, generated config files, transaction files, and runtime state must be identifiable as TunWarden-owned.

Naming examples:

```text
tunwarden0
fwmark 0x... documented range
routing table name/id reserved for TunWarden
nft table inet tunwarden
/run/tunwarden/transactions/<id>.json
/run/tunwarden/*
```

### INV-003: Cleanup must be idempotent

Running cleanup multiple times must be safe.

This applies to:

- TUN deletion,
- route deletion,
- rule deletion,
- nftables cleanup,
- DNS rollback,
- core process termination,
- transaction file cleanup,
- repeated rollback planning from the same transaction file.

### INV-004: Failed connection must not damage direct connectivity

If connection setup fails and strict kill-switch is not enabled, TunWarden must attempt to restore direct connectivity.

### INV-005: NetworkManager connectivity state is not the only health signal

NetworkManager or desktop UI may show limited connectivity while application traffic still works through VPN.

TunWarden must run its own health checks and expose both states separately.

## 5. TUN requirements

### TUN-001: Stable interface name

Use a stable interface name for diagnostics.

Initial candidate:

```text
tunwarden0
```

The current dry-run plan must show the intended stable TUN interface before any execution work is implemented.

### TUN-002: MTU must be configurable

Default MTU should be conservative.

The actual default must be validated during implementation, but the config must allow overrides. The current dry-run plan may show the planner's initial MTU assumption without applying it.

### TUN-003: IPv6 must be explicit

IPv6 must not be accidentally half-enabled.

Initial default:

```text
IPv6 disabled or bypassed until full IPv6 routing/DNS leak handling is implemented.
```

### TUN-004: TUN lifecycle must be daemon-owned

The daemon must own TUN creation and deletion.

The CLI must not create TUN devices directly. The current CLI `plan --mode tun` may describe daemon-owned TUN create steps only as dry-run output unless the daemon is executing a transaction.

## 6. Routing requirements

### RT-001: Use policy routing where appropriate

Full-tunnel mode should use deterministic routing state rather than ad-hoc default route replacement.

The current dry-run planner shows intended policy-rule state before applying anything.

### RT-002: Dedicated routing table

TunWarden should use a dedicated routing table ID/name.

Initial dry-run values:

```text
Table: tunwarden
ID: 51820
```

### RT-003: Route visibility and route-change planning must be inspectable

`tunwarden plan --mode tun <profile>` must show route visibility inputs before any TUN mutation work:

- default IPv4/IPv6 route state;
- default interface when detected;
- server route after resolving hostname servers to a concrete IP address;
- warning when the server route is unknown or would loop through `tunwarden0`.

It must also show intended full-tunnel route and policy-rule desired state without applying it:

- default IPv4 route through the TunWarden routing table;
- policy rule that sends default IPv4 traffic through the TunWarden table;
- VPN server bypass route and policy rule only when the current snapshot provides a concrete server IP;
- blocked/incomplete server-bypass output and warnings when hostname resolution or route lookup does not produce a concrete server IP;
- rollback steps for planned routes and policy rules.

The current dry-run plan must not claim to apply route changes. It is inspectable planner output only.

### RT-004: Default interface must be re-detected

On Wi-Fi reconnect, DHCP change, resume, or default route change, TunWarden must re-detect:

- uplink interface,
- gateway,
- server route,
- DNS state,
- IPv4/IPv6 availability.

### RT-005: Route loop prevention

TunWarden must detect and reject a plan where:

```text
VPN server route -> tunwarden0
```

unless an advanced nested mode is explicitly implemented in the future.

The current dry-run planner must surface this as route-loop risk in human and JSON output.

## 7. DNS requirements

### DNS-001: Prefer systemd-resolved backend on Ubuntu/Debian

Initial DNS integration should use systemd-resolved when available.

The current dry-run plan shows the intended systemd-resolved per-link backend, planned DNS servers, route-only domain, and default-route state before applying anything and warns clearly when `resolvectl` or systemd-resolved state cannot be inspected.

The daemon-owned TUN transaction executor applies the resolved DNS slice only when the plan action is `configure`; blocked DNS plans fail before mutation.

### DNS-002: Do not blindly overwrite /etc/resolv.conf

Direct edits to `/etc/resolv.conf` are not allowed in normal operation.

Fallback handling may be added later, but must be explicit and documented.

### DNS-003: Full-tunnel DNS must be per-link where possible

For full-tunnel mode, TunWarden uses per-link DNS on `tunwarden0` where possible.

Current executor behavior applies the DNS servers already present in `TunDNSPlan.Servers`; the current planner default is `1.1.1.1` until user DNS configuration exists.

```bash
resolvectl dns tunwarden0 <planned-dns-server> [...]
resolvectl domain tunwarden0 '~.'
resolvectl default-route tunwarden0 yes
```

Rollback uses:

```bash
resolvectl revert tunwarden0
```

The executor verifies the link with:

```bash
resolvectl status tunwarden0 --no-pager
```

It requires every planned DNS server and the route-only domain `~.` to be visible after apply.

### DNS-004: Bootstrap DNS must avoid loops

DNS needed to resolve the VPN server must not depend on the VPN tunnel that is not established yet.
