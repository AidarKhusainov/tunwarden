# Testing

This document describes the test commands and GitHub workflows that are supported by the current codebase.

## Default local and PR validation

Run the regular test suite before merging code that changes import, subscription, daemon, or networking behavior:

```bash
go test ./...
```

The default test suite includes:

- unit tests for parsing, planning, rendering, validation, and state persistence;
- provider HTTP integration tests backed by `httptest.Server`;
- Xray JSON subscription fixtures with VLESS profiles plus service outbounds such as `freedom`, `blackhole`, `dns`, and `loopback`;
- CLI HTTP import coverage for provider-like Xray JSON responses;
- CLI-to-daemon control-plane coverage over a real Unix socket with a fake Xray executable;
- daemon TUN lifecycle coverage with fake Xray, fake tun2socks, fake host-network executor, and fake route/TCP/DNS probes.

These tests must not require repository secrets, root privileges, public internet access, a real VPN provider, or mutation of the developer or CI host network.

## Package validation

The main `CI` workflow runs on pull requests and pushes to protected branches. It runs:

- `gofmt` check;
- `go test ./...`;
- `go vet ./...`;
- `govulncheck`;
- Debian package build and install/remove smoke checks;
- generated shell-completion validation.

## Secret-backed TUN acceptance smoke

The `v0.2 acceptance smoke` workflow is the real-world VPN smoke.

It runs only when explicitly requested:

- `workflow_dispatch`;
- or a pull request from the same repository labeled `run-v0.2-acceptance`.

It requires repository secrets for the real test profile and subscription source. It installs real Xray and tun2socks binaries, starts `tunwardend`, connects in TUN mode, checks DNS/connectivity, verifies public egress IP change, disconnects, and uploads sanitized diagnostics.

## Privileged local VPN smoke boundary

There is no separate always-on Docker or network-namespace VPN workflow in the current supported test contract.

TunWarden TUN mode currently verifies system behavior through Linux routing, DNS, nftables, systemd-resolved, child process supervision, and live connectivity probes. Running that as a stable generic Docker or network-namespace CI gate requires an isolated environment that accurately provides those host services. A partial container test that skips or stubs those host services would duplicate the non-root fake-daemon tests and would not prove real TUN behavior.

For pull requests, use `go test ./...` for deterministic provider, daemon, and fake TUN lifecycle coverage. Use the gated `v0.2 acceptance smoke` workflow for real privileged VPN behavior.
