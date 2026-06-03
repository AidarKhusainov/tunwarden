# Roadmap

## 1. Roadmap principles

TunWarden development must prioritize safety before feature breadth.

The roadmap is intentionally staged so that privileged networking is implemented only after planning, rollback, diagnostics, recovery behavior, state ownership, and output redaction are designed and testable.

Important ordering rule:

> Do not add convenience features that hide networking state before the client can reliably connect, disconnect, diagnose, roll back, and recover.

## 2. Current repository state

The repository currently contains a foundation build:

- `tunwarden` CLI skeleton,
- `tunwardend` daemon skeleton,
- local Unix socket HTTP/JSON daemon IPC for read-only status and doctor diagnostics,
- manual systemd service unit with `/run/tunwarden` runtime directory handling and journald logging,
- group-based daemon socket access model for the repository systemd unit,
- read-only `status` with daemon-backed status and conservative local fallback,
- read-only `doctor` with daemon-backed diagnostics and local fallback for platform, command availability, default route/interface, and stale TunWarden-owned resources,
- read-only `logs` with journald-backed daemon log inspection,
- dry-run `recover` command contract,
- initial transaction/profile/subscription models,
- CI with `gofmt` and `go test`,
- canonical documentation under `docs/`.

The repository does **not** yet contain:

- real Xray process management,
- profile import,
- subscription parsing,
- TUN creation,
- route/DNS/nftables mutation,
- `.deb` package or installer,
- auto-start enablement policy.

## 3. Phase 0: Documentation and repository foundation

Status: mostly complete.

Goal: define the project before writing privileged code.

Deliverables:

- documentation index,
- product requirements,
- CLI contract,
- state and security requirements,
- architecture requirements,
- package boundary requirements,
- networking/reliability requirements,
- subscription/profile requirements,
- development guide,
- technical references,
- initial CI skeleton,
- initial CLI/daemon skeleton.

Exit criteria:

- MVP scope is clear.
- Non-goals are documented.
- Networking invariants are documented.
- Recovery and rollback are treated as first-class requirements.
- Filesystem state ownership is documented.
- JSON compatibility and output redaction rules are documented.
- Documentation has one canonical location per concern.

## 4. Phase 1: CLI, daemon, local IPC, and read-only diagnostics foundation

Goal: create the basic process model and diagnostic surface without risky networking changes.

Deliverables:

- `tunwarden` command structure,
- `tunwardend` daemon process,
- local IPC design and implementation,
- restricted daemon API,
- systemd service draft,
- journald logging,
- `status` command,
- `logs` command,
- structured error model,
- JSON output shape for `status`, `doctor`, and `plan`,
- shared redaction helpers for human and JSON output,
- daemon startup recovery scan in read-only mode,
- read-only Linux diagnostics for:
  - default route,
  - default interface,
  - NetworkManager availability/state,
  - systemd-resolved availability/state,
  - DNS mode,
  - nftables availability,
  - stale TunWarden-owned resources.

Implemented foundation subset:

- CLI skeleton,
- daemon skeleton,
- local Unix socket HTTP/JSON transport,
- manual systemd unit for `tunwardend`,
- journald logging through the systemd unit,
- daemon-backed `status` with local fallback,
- daemon-backed `doctor` with local fallback,
- journald-backed `logs` command for daemon logs,
- shared human-output redaction for implemented status, doctor, logs, and recover output,
- read-only local recovery dry-run scan.

Exit criteria:

- CLI can communicate with daemon.
- Daemon can run under systemd.
- Logs are visible through `journalctl` and `tunwarden logs`.
- No privileged networking changes are performed yet.
- Read-only recovery scan can report stale TunWarden-owned state without removing it.
- Default output and `--json` output redact secrets consistently.

## 5. Phase 2: Profile and subscription foundation

Goal: import and normalize profiles before connecting anything.

Deliverables:

- internal profile model,
- manual profile support,
- convenience `tunwarden import` entrypoint,
- XDG-based user config/state layout,
- share link parser for initial protocols,
- Base64 subscription parser,
- subscription storage,
- update diff,
- validation and warnings,
- delete confirmation behavior,
- fixture-based tests.

Initial protocols:

- VLESS,
- VMess,
- Trojan,
- Shadowsocks.

Exit criteria:

- Profiles can be imported, listed, shown, validated, and deleted.
- Subscriptions can be added, listed, shown, updated, and deleted.
- `tunwarden import` can detect supported share links and subscriptions.
- Subscription update failure preserves last known good state.
- Unsupported formats fail clearly.
- Unsafe profile settings are reported as warnings rather than silently accepted.
- Stored user intent follows the documented XDG layout.

## 6. Phase 3: Xray engine lifecycle in proxy-only mode

Goal: start and stop Xray safely without touching system routes, DNS, TUN, or firewall state.

Deliverables:

- Xray engine manager,
- `doctor --core` Xray validation,
- generated runtime config under `/run/tunwarden/`,
- generated core config permissions and atomic writes,
- local SOCKS/HTTP/mixed inbound where supported,
- core process supervision,
- core process privilege minimization,
- graceful stop,
- forced stop,
- core logs,
- basic health check,
- `plan --mode proxy-only`,
- proxy-only `connect` and `disconnect`.

Exit criteria:

- A manual profile can run in proxy-only mode.
- The daemon can stop Xray cleanly.
- Core crash is detected and reported.
- No system routes, DNS, firewall rules, or TUN devices are modified.
- Generated Xray config is runtime output, not persistent source of truth.
- Generated core config is not logged in full by default.
