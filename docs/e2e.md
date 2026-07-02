# Self-hosted E2E

Manual host validation for behavior that is not suitable for the default pull-request gate.

## Run

```text
Actions -> E2E -> Run workflow
```

Default job order:

1. CLI contract
2. Package and service
3. Proxy data-plane
4. Maximum server coverage

## Runner

Required labels:

```text
self-hosted
linux
x64
vpn-e2e
ubuntu-24.04
```

Required host tools:

- systemd;
- `/dev/net/tun`;
- `iproute2`, `nftables`, `resolvectl`, `journalctl`;
- Go from the workflow setup step.

Additional Debian/Ubuntu or arm64 coverage requires dedicated runners or VMs.

## Configuration

Workflow inputs are owned by `.github/workflows/e2e.yml` and the `scripts/e2e/*.sh` entrypoints.
Do not duplicate the full input inventory here.

## Scripts

| Job | Script | Scope |
| --- | --- | --- |
| CLI contract | `scripts/e2e/cli-contract.sh` | CLI command and error checks. |
| Package and service | `scripts/e2e/package-service.sh` | Package install, reinstall, service, cleanup. |
| Proxy data-plane | `scripts/e2e/data-plane.sh` | Proxy connect, egress, listener scope, cleanup. |
| Maximum server coverage | `scripts/e2e/server-coverage.sh` | Real-provider proxy/TUN probes and snapshots. |

## Non-goals

- Not a fork PR gate.
- Not a replacement for unit tests.
- Not permanent release evidence; keep evidence in issues, PRs, or release notes.
