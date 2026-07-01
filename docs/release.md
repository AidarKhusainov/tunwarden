# Release workflow

Reference for tagged GitHub Release automation.

## Trigger

A release is produced only from a pushed semantic version tag:

```text
vMAJOR.MINOR.PATCH
```

Examples: `v0.1.3`, `v0.2.0`.

## Artifacts

For tag `v0.1.3`, the workflow publishes:

```text
podlaz_0.1.3_linux_amd64.deb
podlaz_0.1.3_linux_arm64.deb
SHA256SUMS
```

`podlaz version`, package metadata, artifact names, release notes, and checksums
must all use the same version and commit SHA.

## Validation

Before publication, the workflow validates:

- Go formatting, tests, vet, and vulnerability scan;
- package builds for `amd64` and `arm64`;
- package metadata and contents;
- shell completions;
- binary linkage;
- lintian errors;
- local install, same-version reinstall, and purge cleanup;
- version output for `podlaz` and `plz`;
- man page rendering;
- checksum contents.

Package install validation must confirm that install does not start Xray and does
not change host routing. The package may make `podlazd.service` available through
Debian helper-managed service enable/start behavior.

## Permissions

Use read-only permissions by default. Only the publication job may request
`contents: write`, because GitHub Release creation and asset upload require it.

## Out of scope

- Public apt repository publication.
- Repository signing.
- Starting VPN tunnels.
- Mutating TUN devices, routes, DNS, nftables, firewall rules, or resolver files.
- GUI metadata.
