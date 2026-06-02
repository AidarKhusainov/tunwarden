# Daemon local API

This document defines the implemented v0.1 local daemon API transport and its safety boundary.

The command names, user-visible output, and exit codes are owned by [CLI contract](./cli.md). Runtime paths and daemon state ownership are owned by [State and security requirements](./state-and-security.md).

## MVP transport decision

The v0.1 daemon API uses HTTP/JSON over a Unix domain socket:

```text
/run/tunwarden/tunwardend.sock
```

The runtime directory can be overridden for tests and local development with:

```bash
TUNWARDEN_RUNTIME_DIR=/tmp/tunwarden-dev
```

This keeps the first daemon API small, local-only, and testable with Go's standard library.

## Access model

The v0.1 server sets the daemon socket mode to `0660` and fails startup if permissions cannot be applied.

For local development and manual testing, run both `tunwardend` and `tunwarden status` as the same user and set `TUNWARDEN_RUNTIME_DIR` to a user-owned directory, for example:

```bash
TUNWARDEN_RUNTIME_DIR=/tmp/tunwarden-dev go run ./cmd/tunwardend
TUNWARDEN_RUNTIME_DIR=/tmp/tunwarden-dev go run ./cmd/tunwarden status
```

The packaged system service access model is deferred. It must explicitly define a Unix socket ownership policy, such as a dedicated `tunwarden` group with systemd socket or runtime directory settings. Until that packaging work exists, a root-owned default `/run/tunwarden/tunwardend.sock` may be inaccessible to unprivileged users and the CLI must continue to fall back to read-only local inspection.

Read-only CLI commands must not require root. The packaged daemon access model must preserve that rule before daemon-backed status becomes the only status path.

## Why D-Bus and polkit are deferred

D-Bus and polkit are intentionally not implemented in this issue.

Reasons:

- v0.1 status is read-only and does not require an authorization policy engine;
- there are no privileged route, DNS, nftables, firewall, TUN, or core process mutations in this issue;
- Unix sockets are enough for a local daemon health/status API;
- HTTP/JSON over Unix sockets is simple to unit test without a system bus;
- polkit decisions should be introduced together with real privileged operations and documented authorization rules.

D-Bus and polkit remain valid future options for packaged desktop integration and authorization, but adding them before daemon-owned mutations would increase complexity without improving the current user-visible behavior.

## Implemented endpoint

### `GET /v1/status`

Returns the current daemon-backed status snapshot.

Current v0.1 response shape:

```json
{
  "daemon": "running",
  "connection": "inactive",
  "runtime_directory": "present",
  "proxy": "inactive",
  "tun": "disabled"
}
```

Fields:

| Field | Meaning |
| --- | --- |
| `daemon` | Daemon availability from the daemon's own process. |
| `connection` | Current connection state. v0.1 reports `inactive`. |
| `runtime_directory` | Daemon runtime directory visibility. |
| `proxy` | Proxy lifecycle state. v0.1 reports `inactive`. |
| `tun` | TUN mode state. v0.1 reports `disabled`. |
| `warnings` | Optional daemon-side visibility warnings. |

All listed fields except `warnings` are required in daemon responses. The CLI treats missing required fields as a daemon protocol error and uses the existing warning/fallback path instead of rendering a healthy daemon-backed status.

This is an internal local API contract, not the public `status --json` CLI contract. `tunwarden status --json` remains deferred until the CLI JSON schema is implemented.

## Runtime lifecycle

On startup, `tunwardend`:

1. creates the runtime directory if needed;
2. creates a daemon lock file;
3. removes only a stale Unix socket at the socket path if present;
4. fails explicitly when the socket path exists but is not a Unix socket;
5. listens on the Unix socket;
6. applies the socket mode;
7. serves the read-only status endpoint.

On graceful shutdown, `tunwardend`:

1. shuts down the HTTP server;
2. closes the Unix socket listener;
3. removes the socket path;
4. removes the lock file.

If another daemon appears to be running or a previous shutdown left an unclean lock file, startup fails explicitly instead of silently taking over daemon-owned state.

## Safety boundary

The v0.1 daemon API is local-only and read-only.

It must not:

- create or delete TUN interfaces;
- add, remove, or replace routes;
- change DNS configuration;
- create, modify, flush, or delete nftables or firewall state;
- start, stop, or supervise Xray;
- mutate user profiles or subscriptions.

The current endpoint only reports daemon availability and conservative inactive runtime state.
