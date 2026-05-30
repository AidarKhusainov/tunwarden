# Roadmap

## 1. Roadmap principles

TunWarden development should prioritize safety before feature breadth.

The roadmap is intentionally staged so that privileged networking is implemented only after planning, rollback, and diagnostics are designed.

## Phase 0: Documentation and repository foundation

Goal: define the project before writing privileged code.

Deliverables:

- documentation index,
- product requirements,
- architecture requirements,
- networking/reliability requirements,
- subscription/profile requirements,
- contribution guidelines later,
- initial issue labels later,
- CI skeleton later.

Exit criteria:

- The MVP scope is clear.
- Non-goals are documented.
- Networking invariants are documented.
- Panic reset and rollback are treated as first-class requirements.

## Phase 1: CLI and daemon skeleton

Goal: create the basic process model without risky networking changes.

Deliverables:

- `tunwarden` CLI skeleton,
- `tunwardend` daemon skeleton,
- local IPC design,
- systemd unit draft,
- journald logging,
- status command,
- version command,
- structured error model.

Exit criteria:

- CLI can communicate with daemon.
- Daemon can run under systemd.
- Logs are visible through `journalctl` and `tunwarden logs`.
- No privileged networking changes are performed yet.

## Phase 2: Profile and subscription foundation

Goal: import and normalize profiles before connecting anything.

Deliverables:

- internal profile model,
- manual profile support,
- share link parser for initial protocols,
- Base64 subscription parser,
- subscription storage,
- update diff,
- validation and warnings,
- fixture-based tests.

Initial protocols:

- VLESS,
- VMess,
- Trojan,
- Shadowsocks.

Exit criteria:

- Profiles can be imported, listed, shown, validated, and deleted.
- Subscription update failure preserves last known good state.
- Unsupported formats fail clearly.

## Phase 3: Xray engine lifecycle

Goal: start and stop Xray safely in proxy-only mode.

Deliverables:

- Xray engine manager,
- generated runtime config,
- local SOCKS/HTTP/mixed inbound where supported,
- core process supervision,
- graceful stop,
- forced stop,
- core logs,
- basic health check.

Exit criteria:

- A manual profile can run in proxy-only mode.
- The daemon can stop Xray cleanly.
- Core crash is detected and reported.
- No system routes/DNS/firewall are modified.

## Phase 4: Network planner and dry-run

Goal: design network changes without applying them.

Deliverables:

- system snapshot model,
- route planner,
- DNS planner,
- firewall planner,
- transaction model,
- `tunwarden plan <profile>`,
- planner unit tests.

Exit criteria:

- A full-tunnel plan can be generated from a fake system snapshot.
- Plan output is readable by technical users.
- Planner can detect route loop risk.
- Planner can produce rollback steps.

## Phase 5: Safe TUN MVP

Goal: implement full-tunnel mode with rollback.

Deliverables:

- TUN interface creation,
- routing table/rule apply,
- systemd-resolved DNS apply,
- nftables foundation,
- transaction apply/commit/rollback,
- `panic-reset`,
- `doctor`,
- integration tests in network namespaces where possible.

Exit criteria:

- Failed connection attempts roll back.
- Disconnect leaves no TunWarden-owned routes/rules/DNS/nftables state.
- `panic-reset` works when disconnected and after simulated failure.
- VPN server route bypasses TUN.

## Phase 6: Laptop reliability

Goal: make the client robust on real laptops.

Deliverables:

- NetworkManager event handling,
- default route/interface change handling,
- DNS change handling,
- suspend/resume hooks,
- reconnect state machine,
- rate-limited retries,
- health checks after resume,
- Ubuntu LTS manual test checklist.

Exit criteria:

- Active connection recovers after suspend/resume.
- Active connection recovers after Wi-Fi reconnect.
- DHCP/DNS changes are handled without stale state.
- NetworkManager limited connectivity is reported but not blindly treated as VPN failure.

## Phase 7: Packaging

Goal: make installation and service management reliable.

Deliverables:

- `.deb` package,
- systemd units,
- default config files,
- uninstall cleanup policy,
- shell completions,
- man pages later,
- Fedora/Arch packaging plan.

Exit criteria:

- Fresh Ubuntu installation can install, start daemon, connect, disconnect, and uninstall safely.

## Phase 8: Advanced features

Goal: add convenience only after reliability is solid.

Candidates:

- latency test,
- URL test,
- auto-select,
- split tunnel,
- routing rule DSL,
- IPv6 full support,
- AmneziaWG engine,
- GUI client,
- provider-specific subscription metadata,
- auto-update core with verification.

## Explicit deferrals

The following should not be started until the earlier reliability phases are strong:

- GUI,
- mobile clients,
- complex visual routing editor,
- router mode,
- plugin system,
- broad non-Xray protocol expansion.

## First milestone proposal

A realistic first public milestone:

```text
v0.1.0: proxy-only technical preview
```

Features:

- CLI + daemon,
- manual profile import,
- Base64 subscription import,
- Xray proxy-only mode,
- status/logs/doctor basics,
- no TUN mode yet.

Second milestone:

```text
v0.2.0: safe TUN preview
```

Features:

- TUN full-tunnel,
- transaction rollback,
- systemd-resolved backend,
- nftables foundation,
- panic-reset,
- Ubuntu LTS test checklist.
