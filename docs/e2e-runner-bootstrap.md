# E2E runner bootstrap

This document defines the automated bootstrap path for the dedicated self-hosted E2E runner described in [Self-hosted E2E validation](./e2e.md).

The bootstrap workflow provisions one clean Ubuntu 24.04 x86_64 server into a GitHub Actions self-hosted runner with the labels required by `.github/workflows/e2e.yml`.

## Workflow

Run the workflow from GitHub Actions:

```text
Actions -> Provision E2E runner -> Run workflow
```

Inputs:

| Input | Default | Purpose |
| --- | --- | --- |
| `host` | required | SSH host or IP address of the clean server. |
| `ssh_user` | `ubuntu` | SSH user. The user must be `root` or have passwordless sudo. |
| `ssh_port` | `22` | SSH port. |
| `runner_name` | empty | Optional GitHub runner name. Empty means `podlaz-<hostname>-vpn-e2e`. |
| `runner_user` | `gha-runner` | Local Linux user that owns the runner service. |
| `runner_labels` | `self-hosted,linux,x64,vpn-e2e,ubuntu-24.04` | Runner labels consumed by the E2E workflow. |
| `reset_runner` | `true` | Stop and replace any existing installation in `/opt/actions-runner`. |

## Required GitHub configuration

The provisioning job uses the `vpn-e2e` environment so the bootstrap values can be environment-scoped instead of repository-wide.

| Name | Purpose |
| --- | --- |
| `E2E_BOOTSTRAP_SSH_PRIVATE_KEY` | SSH key material used by the GitHub-hosted provisioning job to connect to the new server. |
| `E2E_RUNNER_ADMIN_TOKEN` | GitHub token that can create repository self-hosted runner registration tokens. |

Optional value:

| Name | Purpose |
| --- | --- |
| `E2E_BOOTSTRAP_SSH_KNOWN_HOSTS` | Pinned SSH known-hosts entry for the target server. If omitted, the workflow uses `ssh-keyscan` for first-time bootstrap convenience. |

Use a token scoped to this repository with the minimum administration permission needed to create runner registration tokens.

## Server contract

The remote bootstrap script intentionally targets a narrow host class:

- Ubuntu 24.04;
- x86_64 architecture;
- systemd as PID 1;
- `/dev/net/tun` available;
- SSH user can execute `sudo` without an interactive password.

The script installs host packages required by the existing E2E suites, creates the runner service user, creates the `podlaz` access group used by socket-access tests, installs a narrow sudoers policy, downloads the latest GitHub Actions runner release, configures it against this repository, and starts it as a systemd service.

## Sudo policy installed by bootstrap

The workflow installs `/etc/sudoers.d/podlaz-e2e-runner` for the runner user.

The policy allows root execution of the commands used by the current E2E scripts:

```text
/usr/bin/true
/usr/bin/apt
/usr/bin/systemctl
/usr/bin/journalctl
/usr/sbin/ip
/usr/sbin/nft
/usr/bin/kill
/usr/bin/env
```

It also allows the runner user to execute `/usr/bin/env` with the `podlaz` run-as group so daemon socket access can be tested without making the runner permanently privileged beyond the selected command path.

## Recommended first run

After provisioning a clean server:

1. Run `Provision E2E runner` with the target host.
2. Wait for the runner to appear online in repository settings.
3. Run `E2E` with `suite=cli-contract`.
4. Run `E2E` with `suite=package-service`.
5. Add the real VPN E2E values to the `vpn-e2e` environment.
6. Run `real-vpn` first with `lifecycle=false` and `tun=false`.
7. Enable `lifecycle=true`, then enable `tun=true` only on a disposable host.

## Safety boundary

The provisioning workflow must stay `workflow_dispatch` only. It must not run on `pull_request`, because it creates infrastructure capable of running privileged networking tests.

The E2E workflow itself remains manual and self-hosted only. The bootstrap flow only removes the manual server preparation step; it does not make privileged VPN lifecycle or TUN suites safe as automatic PR gates.
