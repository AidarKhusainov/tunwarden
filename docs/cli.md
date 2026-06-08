# CLI Contract

This document is the canonical command-line interface contract for TunWarden.

## Plan

Implemented plan commands:

```bash
tunwarden plan --mode proxy-only <profile-id> [--json]
tunwarden plan --mode tun <profile-id> [--json]
```

`plan --mode proxy-only` is read-only and explains generated proxy-only runtime details without starting Xray or changing system networking.

`plan --mode tun` is currently a read-only system snapshot preview. It explains the current default route, default interface, route to the VPN server candidate, DNS mode, NetworkManager state, nftables state, IPv4/IPv6 assumptions, known TunWarden TUN device state, and stale TunWarden-owned resources. Hostname servers are resolved read-only with a timeout before server route lookup.

The implemented TUN plan must not create TUN devices, change routes, change DNS, change nftables/firewall state, start Xray, or write runtime config files. It does not yet show final intended TUN apply steps, rollback steps, kill-switch behavior, or health-check behavior.

Future full TUN planning must show intended TUN, route, DNS, nftables, rollback, and health-check behavior without applying anything. `connect --mode tun` must apply changes only through daemon-owned network transactions.

## JSON

Plan JSON uses `schema_version`, `status`, `warnings`, `errors`, `mode`, `plan`, `steps`, and `rollback_steps`. Human and JSON output must apply the same redaction policy.

## Safety

Read-only commands must not require root. Commands that remove state or execute cleanup must require explicit long flags such as `--execute` and `--yes`.
