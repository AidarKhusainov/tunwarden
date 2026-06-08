# System Snapshot Model

TunWarden TUN planning starts from a read-only snapshot of the host networking state. The snapshot is an input to planners, not an executor and not a transaction.

## Scope

The snapshot layer may inspect:

- default IPv4 and IPv6 routes;
- the interface and gateway selected by the default route;
- the route to the VPN server candidate;
- systemd-resolved availability and status;
- NetworkManager availability and advisory state;
- nftables availability and the `inet tunwarden` table presence;
- known TunWarden TUN device names such as `tunwarden0`;
- TunWarden-owned resources that would be stale before a future TUN apply step.

The snapshot layer must not create, update, or delete TUN devices, routes, DNS configuration, nftables tables, firewall rules, processes, or runtime files.

## Server route resolution

`profile.Profile.Server` may be an IP literal or a hostname. For IP literals, the snapshot collector may run a read-only route lookup directly against that IP. For hostnames, the collector must first resolve the hostname under a bounded context timeout and then run the route lookup against a resolved IP address.

DNS resolution failures, empty DNS answers, and DNS timeouts must not fail the whole snapshot. They must produce an `unknown` server-route observation with a clear detail message so planners can warn about incomplete server-bypass visibility.

## Status vocabulary

Every optional observation uses this vocabulary:

| Status | Meaning |
| --- | --- |
| `detected` | The observation completed and found the requested state. |
| `missing` | The command, backend, route, table, or device was not present. |
| `unsupported` | The current platform is outside the implemented Linux snapshot scope. |
| `unknown` | The collector attempted a read-only observation but could not classify the result safely. |

This vocabulary lets planners distinguish host limitations from stale state and from incomplete visibility.

## Planner contract

`tunwarden plan --mode tun <profile-id>` consumes a snapshot and produces an inspectable read-only plan. The plan may warn about incomplete visibility, missing optional backends, missing IPv6 support, stale TunWarden-owned resources, DNS resolution failure, or a server route that would loop through `tunwarden0`.

The TUN snapshot plan has no rollback steps because it does not mutate state. Future TUN execution work must still build a real transaction with before snapshot, desired plan, apply steps, verification, rollback steps, and recovery ownership.

## Fake snapshots

Planner tests should use fake snapshots for common desktop topologies. Fake snapshots must cover at least:

- a systemd-resolved + NetworkManager + nftables desktop with no stale TunWarden resources;
- a desktop where optional tools such as `resolvectl`, `nmcli`, or `nft` are missing;
- a desktop with stale TunWarden-owned resources such as `tunwarden0` and `table inet tunwarden`.
