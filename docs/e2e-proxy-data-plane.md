# Proxy data-plane E2E job

The `E2E` workflow now runs four jobs in order:

1. `CLI contract e2e`
2. `Package and service e2e`
3. `Proxy data-plane e2e`
4. `Maximum server coverage e2e`

The workflow dependency chain is intentional: `package-service` needs `cli-contract`, `data-plane` needs `package-service`, and `server-coverage` needs `data-plane`. Later jobs only run after earlier release/acceptance checks pass.

## Job 3: Proxy data-plane

Script:

```bash
bash scripts/e2e/data-plane.sh
```

Scope:

- imports one configured real profile into isolated user state;
- builds and installs the local Debian package for the native runner architecture;
- starts `podlazd.service`;
- connects with explicit `--mode proxy-only` and validates SOCKS and HTTP proxy egress;
- connects with default `connect <profile-id>` and repeats SOCKS and HTTP proxy egress checks;
- verifies proxy listeners are loopback-only;
- verifies SOCKS and HTTP proxy traffic fails after disconnect;
- runs `status` and recovery dry-run after disconnect to detect podlaz-owned stale state;
- when `PODLAZ_E2E_RELIABILITY_CYCLES` is greater than zero, repeats the proxy-only lifecycle that many times, including listener checks, SOCKS/HTTP egress, cleanup, status, and recovery dry-run checks in every cycle.

`PODLAZ_E2E_RELIABILITY_CYCLES` defaults to `0` in the workflow and is intended to be set to `100` for release evidence when repeated lifecycle coverage is required.
