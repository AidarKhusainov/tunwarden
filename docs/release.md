# Release workflow

This document defines podlaz's GitHub Release automation contract.

The release workflow publishes versioned GitHub Release artifacts only. Public apt repository publication and package repository signing remain out of scope for this workflow.

## Trigger

A release is produced only from a semantic version tag pushed to the repository.

Required tag format:

```text
vMAJOR.MINOR.PATCH
```

Examples:

```text
v0.1.3
v0.2.0
```

The workflow intentionally has no manual tag input. To release a version, create and push the corresponding Git tag.

## Version mapping

For a tag such as `v0.1.3`:

| Value | Mapping |
| --- | --- |
| Git tag | `v0.1.3` |
| Binary version shown by `podlaz version` | `0.1.3` |
| Debian package version | `0.1.3-1` |
| Binary tarball | `podlaz_0.1.3_linux_amd64.tar.gz` |
| Debian package | `podlaz_0.1.3-1_amd64.deb` |
| Checksums | `podlaz_0.1.3_checksums.txt` |

The workflow passes the full Debian package version to the package build script as `PODLAZ_DEB_VERSION`. The package manifest sets `version_schema: none` so nFPM preserves the Debian package version string exactly instead of treating the `-1` suffix as a semantic-version prerelease and normalizing it into a tilde-qualified Debian version.

## Artifacts

The release workflow publishes:

- a Linux `amd64` binary tarball containing `podlaz` and `podlazd`;
- a local installable Debian package for `amd64`;
- a SHA-256 checksum file covering every downloadable artifact produced by the workflow.

`arm64` remains supported by the local package build script where the build host can produce it, but it is not part of the first release automation gate.

## Validation gate

Before publication, the workflow runs regular Go checks, vulnerability scanning, package build, package metadata inspection, package content inspection, package linting, local package installation, version validation, and manual page rendering validation.

Package install validation also checks that the package does not start `podlazd` and that the host route table is unchanged by package installation.

Systemd lifecycle assertions that require systemd as PID 1 remain VM or systemd-capable host validation work and are not claimed by the container-backed release workflow.

## Workflow permissions

The workflow declares read-only top-level permissions. Build and validation jobs use read-only repository access. Only the final publication job grants `contents: write`, because GitHub Release creation and asset upload require write access to repository contents.

## Action pinning policy

The workflow uses official GitHub-owned Actions:

- `actions/checkout@v4`
- `actions/setup-go@v5`
- `actions/upload-artifact@v4`
- `actions/download-artifact@v4`

These are tag-pinned rather than SHA-pinned because they are first-party GitHub Actions with stable major-version release channels and a lower supply-chain risk than third-party marketplace actions. Any future third-party Action added to the release path must be pinned to a full-length commit SHA unless the PR explicitly documents why that is not practical.

## Release notes

Generated release notes include:

- the exact Git tag;
- the exact commit SHA;
- the names of all published artifacts;
- the local Debian package install command;
- the package auto-start policy.

Curated human release notes can be added by editing the GitHub Release after publication or by extending the workflow in a later PR.

## Safety boundary

The release workflow must not:

- create a public apt repository;
- sign repository metadata;
- add broad installer scripts;
- enable or start `podlazd.service` during package installation;
- start a VPN tunnel;
- mutate TUN devices, routes, DNS, nftables, firewall rules, or host resolver files.
