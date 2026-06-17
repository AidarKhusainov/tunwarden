# Profile validation

`tunwarden profile validate <profile-id> [--mode proxy-only|tun] [--json]` checks a stored profile without starting Xray, contacting `tunwardend`, requiring root, or mutating TUN, routes, DNS, nftables, firewall, runtime config, or daemon state.

The command validates the normalized profile fields and whether the selected backend and mode can render the profile into the supported Xray runtime configuration subset.

The default mode is `proxy-only`, matching `tunwarden connect`. Use `--mode tun` to check whether the same profile can be rendered for the TUN-mode Xray configuration path.

Human output reports the profile name, profile ID, source, mode, backend, protocol, and validation status. Sensitive profile values are redacted.

`--json` returns the common top-level shape with `schema_version: "v1"`, `status`, `warnings`, `errors`, the redacted profile, selected `mode`, selected `backend`, and a boolean `valid` field.

Exit codes:

| Code | Meaning |
| ---: | --- |
| 0 | Profile is valid for the selected mode/backend. |
| 1 | Profile lookup or profile store access failed. |
| 2 | Invalid command usage, unsupported flags, or unsupported validation mode. |
| 3 | Profile was found but cannot be rendered for the selected mode/backend. |

This command is read-only. It is intended for diagnostics and automation before `plan` or `connect`.
