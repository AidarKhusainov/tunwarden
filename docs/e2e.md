# Self-hosted E2E validation

This document defines the staged self-hosted E2E plan for podlaz.

The E2E workflow is intentionally manual and self-hosted only. It must not run on `pull_request` because the repository is public and the runner can mutate host networking when privileged suites are enabled.

## Runner contract

The default runner must be a dedicated Ubuntu 24.04 x64 host with these labels:

```text
self-hosted
linux
x64
vpn-e2e
ubuntu-24.04
```

Native package/service and maximum server-coverage suites can also target additional dedicated runners by selecting workflow labels:

```text
platform_label: ubuntu-24.04 | ubuntu-22.04 | debian-12 | debian-13
arch_label: x64 | arm64
```

A selected native runner must have matching GitHub runner labels and a matching Debian package architecture. The scripts intentionally fail instead of installing a cross-built package on the wrong CPU architecture.

The host must provide:

- systemd as the service manager;
- `/dev/net/tun`;
- `iproute2`;
- `nftables`;
- `systemd-resolved` and `resolvectl`;
- `journalctl`;
- Go 1.26.4 through the workflow setup step;
- passwordless sudo only for the E2E operations selected by the suite.

## Workflow

Run the workflow from GitHub Actions:

```text
Actions -> E2E -> Run workflow
```

Inputs:

| Input | Values | Purpose |
| --- | --- | --- |
| `suite` | `cli-contract`, `package-service`, `real-vpn`, `real-vpn-extended`, `server-coverage`, `all` | Selects the E2E tier. |
| `platform_label` | `ubuntu-24.04`, `ubuntu-22.04`, `debian-12`, `debian-13` | Selects the self-hosted OS label for native package/service and server-coverage runs. |
| `arch_label` | `x64`, `arm64` | Selects the self-hosted CPU architecture label for native package/service and server-coverage runs. |
| `lifecycle` | boolean | Allows `real-vpn` to run daemon connect/disconnect checks. |
| `tun` | boolean | Allows real VPN suites to apply TUN-mode networking mutations. |
| `crash_tests` | boolean | Allows real VPN extended/server-coverage suites to kill supervised core and daemon processes. |
| `host_disruption_tests` | boolean | Allows `server-coverage` to execute host-provided suspend/resume, network reconnect, DHCP renew, DNS mutation, and polkit wrapper probes. |
| `stability_minutes` | integer string | Long-running stability probe duration for `real-vpn-extended` and `server-coverage`. |

The default suite is `cli-contract`. The lifecycle, TUN, crash, host-disruption, and long-running flags default to disabled values so a manual dispatch cannot accidentally change host routes, DNS, nftables, TUN state, daemon/core process state, or host connectivity.

## Tier 1: CLI contract

Script:

```bash
bash scripts/e2e/cli-contract.sh
```

Scope:

- builds a local `podlaz` binary from the checked-out commit;
- uses isolated `XDG_CONFIG_HOME`, `XDG_STATE_HOME`, and `XDG_CACHE_HOME`;
- exercises help and version behavior;
- exercises shell completion generation for bash, zsh, and fish;
- exercises profile add/import/list/show/validate/delete paths;
- exercises local import detection for Xray JSON, plain URI-list, and Base64 URI-list fixtures;
- exercises subscription add/update/list/show/delete paths with a local `file://` fixture;
- exercises read-only plan output for proxy-only and TUN modes;
- exercises deferred JSON and invalid-argument exit-code gates for daemon-backed commands;
- writes stdout/stderr diagnostics for every command to the workflow artifact directory.

This tier does not require VPN credentials and does not intentionally mutate host networking.

## Tier 2: Package and service

Script:

```bash
bash scripts/e2e/package-service.sh
```

Scope:

- installs the pinned package toolchain from `packaging/package-toolchain.env`;
- builds the local Debian package for the native runner architecture;
- fails fast if `PODLAZ_DEB_ARCH` does not match the host `dpkg --print-architecture`;
- inspects package metadata and contents;
- installs the package locally;
- checks installed CLI, daemon, completions, systemd unit, and sysusers files;
- starts and stops `podlazd.service` under systemd;
- captures `systemctl status` and journald diagnostics;
- validates same-version reinstall and package removal.

This tier requires passwordless sudo for package and service operations, including `apt`, `systemctl`, and `journalctl`.

## Tier 3: Real VPN

Script:

```bash
bash scripts/e2e/real-vpn.sh
```

Required environment secret:

```text
PODLAZ_E2E_PROFILE_URI
```

Optional environment secrets and variables:

```text
PODLAZ_E2E_EXPECTED_EGRESS_IP
PODLAZ_E2E_PUBLIC_IP_CHECK_URL
PODLAZ_E2E_DNS_CHECK_HOST
```

Default behavior with `lifecycle=false`:

- imports the real E2E profile into isolated user state;
- validates proxy-only and TUN renderability;
- renders proxy-only and TUN plans;
- verifies that TUN planning remains dry-run.

Lifecycle behavior with `lifecycle=true`:

- builds and installs the local Debian package;
- starts `podlazd.service`;
- runs proxy-only connect/status/disconnect through the daemon socket access model;
- captures sanitized systemd, journal, route, rule, DNS, and nftables diagnostics.

TUN behavior with both `lifecycle=true` and `tun=true`:

- runs `connect --mode tun`;
- checks DNS and public egress;
- disconnects;
- runs recovery inspection and explicit cleanup;
- captures sanitized diagnostics.

Only enable `tun=true` on a disposable or dedicated test host. TUN mode is expected to mutate host networking while the transaction is active.

## Tier 4: Extended real VPN

Script:

```bash
bash scripts/e2e/real-vpn-extended.sh
```

Required environment secret:

```text
PODLAZ_E2E_PROFILE_URI
```

Optional environment secrets for broader provider/protocol coverage:

```text
PODLAZ_E2E_PROFILE_URI_2
PODLAZ_E2E_PROFILE_URI_3
PODLAZ_E2E_PROFILE_URI_4
PODLAZ_E2E_SUBSCRIPTION_URL
PODLAZ_E2E_EXPECTED_EGRESS_IP
PODLAZ_E2E_EXPECTED_EGRESS_IPV6
```

Optional environment variables:

```text
PODLAZ_E2E_EXPECT_IPV6=observe|blocked|egress
PODLAZ_E2E_PUBLIC_IP_CHECK_URL
PODLAZ_E2E_PUBLIC_IPV6_CHECK_URL
PODLAZ_E2E_DNS_CHECK_HOST
```

Scope:

- imports up to four real profile secrets into isolated user state;
- validates and plans every imported profile for proxy-only and TUN modes;
- builds and installs the local Debian package;
- starts `podlazd.service`;
- optionally exercises a real subscription URL;
- runs proxy-only connect/status/disconnect for every imported profile;
- runs concurrent `status` probes while a connection is active;
- verifies that a second connect is rejected while a connection is active;
- runs idempotent disconnect checks;
- with `tun=true`, runs TUN connect/status/DNS/public-egress/disconnect/recover for every imported profile;
- with `crash_tests=true`, kills the supervised Xray core process and validates status/doctor/disconnect/recover behavior;
- with `crash_tests=true`, kills `podlazd.service`, restarts it, and validates status/recover/disconnect behavior;
- with `stability_minutes > 0`, keeps one profile connected and polls status, DNS, and public egress repeatedly;
- records host snapshots before, during, and after lifecycle operations.

This tier intentionally does not execute destructive host-disruption actions such as `systemctl suspend`, NetworkManager reconnect, or DHCP renew. Those actions can strand a remote CI runner and need a provider-specific host with out-of-band recovery. Use the `server-coverage` tier with host wrappers for those checks.

## Tier 5: Maximum server coverage

Script:

```bash
bash scripts/e2e/server-coverage.sh
```

At least one of these secrets must be present:

```text
PODLAZ_E2E_PROFILE_URI
PODLAZ_E2E_PROFILE_URI_LIST
```

Optional environment secrets for broad provider/protocol coverage:

```text
PODLAZ_E2E_PROFILE_URI_2
PODLAZ_E2E_PROFILE_URI_3
PODLAZ_E2E_PROFILE_URI_4
PODLAZ_E2E_PROFILE_URI_LIST
PODLAZ_E2E_SUBSCRIPTION_URL
PODLAZ_E2E_EXPECTED_EGRESS_IP
PODLAZ_E2E_EXPECTED_EGRESS_IPV6
```

`PODLAZ_E2E_PROFILE_URI_LIST` is a newline-delimited secret for additional real profile URIs. Use it to cover more than four representative providers/protocols without adding new workflow inputs.

Optional environment variables:

```text
PODLAZ_E2E_EXPECT_IPV6=observe|blocked|egress
PODLAZ_E2E_PUBLIC_IP_CHECK_URL
PODLAZ_E2E_PUBLIC_IPV6_CHECK_URL
PODLAZ_E2E_DNS_CHECK_HOST
PODLAZ_E2E_STATUS_CONCURRENCY
PODLAZ_E2E_HOST_WRAPPER_DIR
PODLAZ_E2E_HOST_WRAPPER_TIMEOUT_SECONDS
PODLAZ_E2E_HOST_DISRUPTION_MODE=proxy-only|tun
```

Scope:

- imports numbered profile secrets and every non-empty line from `PODLAZ_E2E_PROFILE_URI_LIST` into isolated user state;
- validates and plans every imported profile for proxy-only and TUN modes;
- builds and installs the local Debian package for the native runner architecture;
- starts `podlazd.service`;
- optionally exercises a real subscription URL;
- runs proxy-only connect/status/DNS/disconnect/idempotent-disconnect for every imported profile;
- runs concurrent status probes, overlapping second-connect rejection, and disconnect during active status polling;
- with `tun=true`, runs TUN connect/status/DNS/public-egress/IPv6 observation/disconnect/recover for every imported profile;
- with `crash_tests=true`, kills supervised Xray and `podlazd.service`, then validates status/doctor/recover/disconnect behavior;
- with `host_disruption_tests=true`, keeps a connection active and runs host-provided wrappers for suspend/resume, network reconnect, DHCP renewal, DNS change, and polkit GUI/TTY authorization probes;
- with `stability_minutes > 0`, keeps one profile connected and polls status, DNS, and public egress repeatedly;
- records host snapshots before, during, and after lifecycle, crash, host-disruption, and cleanup operations.

### Host-disruption wrappers

The workflow never runs raw `systemctl suspend`, NetworkManager reconnect, DHCP renew, resolver mutation, or interactive polkit commands directly. When `host_disruption_tests=true`, `server-coverage.sh` looks for root-owned host wrappers in `PODLAZ_E2E_HOST_WRAPPER_DIR`, defaulting to:

```text
/usr/local/libexec/podlaz-e2e
```

Supported wrapper names are fixed:

```text
suspend-resume
network-reconnect
dhcp-renew
dns-change
polkit-gui-auth
polkit-tty-auth
```

The script runs only executable wrappers with those names, through `sudo -n timeout`. Missing wrappers are recorded in diagnostics. Enabling host-disruption tests with no supported wrappers is a failure, because it would otherwise create a false positive.

Each wrapper is responsible for host-specific safety, out-of-band recovery, and restoring any temporary state it changes. The E2E script provides these environment values to wrappers:

```text
PODLAZ_E2E_DNS_CHECK_HOST
PODLAZ_E2E_ACTIVE_MODE
```

After every wrapper, the script captures system diagnostics and re-checks podlaz status, doctor output, DNS resolution, and TUN egress when running in TUN mode.

## Coverage boundaries

The self-hosted server can cover real package/service, daemon, proxy-only, TUN, DNS, egress, crash, concurrency, multiple-profile, multiple-provider/subscription, selected platform/architecture, and long-running behavior.

The following items require explicit host support beyond the default single remote host:

| Area | Status |
| --- | --- |
| Suspend/resume | Covered by `server-coverage` only when the host provides a safe `suspend-resume` wrapper and out-of-band recovery. |
| Wi-Fi/network reconnect | Covered by `server-coverage` only when the host provides a safe `network-reconnect` wrapper for its NetworkManager/device topology. |
| DHCP renewal | Covered by `server-coverage` only when the host provides a safe `dhcp-renew` wrapper. |
| DNS change while connected | Covered by regular diagnostics/resolution checks; active resolver mutation requires the `dns-change` wrapper. |
| NetworkManager behavior | Diagnostics are captured when `nmcli` is available; destructive reconnect requires a wrapper. |
| Debian/Ubuntu matrix beyond this host | Covered by selecting `platform_label` only when matching self-hosted runners exist. |
| arm64 package/service install | Covered by selecting `arch_label=arm64` only when a native arm64 self-hosted runner exists. |
| Polkit GUI/TTY authorization flows | Covered by `server-coverage` only when the host provides `polkit-gui-auth` or `polkit-tty-auth` wrappers with an appropriate agent/session. |
| All protocols | Requires representative profile secrets; use numbered profile secrets plus `PODLAZ_E2E_PROFILE_URI_LIST` for additional providers/protocols. |

## Suggested sudoers progression

Start with the smallest policy needed for the selected tier.

Smoke/CLI host preflight:

```text
gha-runner ALL=(root) NOPASSWD: /usr/bin/true, /usr/sbin/ip, /usr/sbin/nft
```

Package/service tier:

```text
gha-runner ALL=(root) NOPASSWD: /usr/bin/apt, /usr/bin/systemctl, /usr/bin/journalctl, /usr/bin/env
```

Extended crash tier, only when `crash_tests=true`:

```text
gha-runner ALL=(root) NOPASSWD: /usr/bin/kill
```

Server host-disruption tier, only when `host_disruption_tests=true`:

```text
gha-runner ALL=(root) NOPASSWD: /usr/bin/timeout, /usr/bin/env, /usr/local/libexec/podlaz-e2e/suspend-resume, /usr/local/libexec/podlaz-e2e/network-reconnect, /usr/local/libexec/podlaz-e2e/dhcp-renew, /usr/local/libexec/podlaz-e2e/dns-change, /usr/local/libexec/podlaz-e2e/polkit-gui-auth, /usr/local/libexec/podlaz-e2e/polkit-tty-auth
```

Real VPN lifecycle and TUN tiers need a narrower project-owned sudo wrapper before they become routine gates. Until then, run them manually and review the diagnostics after every run.

## Diagnostics

Each suite writes command stdout/stderr and host diagnostics to `${RUNNER_TEMP}/podlaz-e2e-artifacts`, which the workflow uploads as an artifact.

Diagnostics must stay sanitized. Do not upload full share URIs, subscription URLs, generated Xray configs, private keys, provider tokens, authorization headers, or unredacted credentials.

## Non-goals

- The E2E workflow is not a required PR gate.
- The real VPN suite is not safe for forked pull-request code.
- The lifecycle/TUN/crash/host-disruption tiers are not enabled by default.
- E2E diagnostics are not permanent release evidence; release evidence belongs in the relevant release issue, pull request, or release notes.
