# Subscription client identity

This document owns the privacy and safety contract for subscription request identity behavior.

## Current request behavior

HTTP(S) subscription fetches always send the explicit TunWarden product `User-Agent`.

TunWarden does not guess provider-specific HWID, device-id, or client-identity field names. If an HTTP(S) subscription URL contains the exact placeholder `{tunwarden-client-id}`, TunWarden replaces that placeholder with its stable privacy-safe client ID before sending the request.

The subscription URL owns the wire location and field name. For example, a provider URL can use the placeholder in a query parameter such as `?hwid={tunwarden-client-id}` or `?device_id={tunwarden-client-id}`. TunWarden does not add either parameter by itself.

If the placeholder is absent, TunWarden does not generate, read, or send the client ID for that fetch.

## Client ID storage

The client ID is a random TunWarden value generated on first placeholder use and persisted in user-owned state:

```text
$XDG_STATE_HOME/tunwarden/client-id
fallback: ~/.local/state/tunwarden/client-id
```

The state directory is created with private permissions where practical, for example `0700`, and the identity file is private to the user, for example `0600`.

The value is stable across `tunwarden import <http-url>` and `tunwarden subscription update` because both use the shared HTTP(S) subscription fetch path.

## Privacy contract

The implementation must not read or send raw host identifiers, including:

- `/etc/machine-id`;
- MAC addresses;
- hostname;
- DMI serials;
- disk serials;
- CPU identifiers;
- other raw hardware or installation identifiers.

The client ID must be treated as sensitive subscription identity. It must not be printed in full in human output, JSON output, logs, errors, tests, fixtures, issue comments, or pull request examples.

## Reset behavior

Resetting the client ID is explicit file removal by the user. Removing only `$XDG_STATE_HOME/tunwarden/client-id` or the fallback `~/.local/state/tunwarden/client-id` resets the generated subscription client identity.

Reset must not remove profile state, subscription state, daemon state, runtime state, generated core configuration, or networking state. Resetting the client ID can consume a new provider device slot or break provider-side device binding when a provider enforces device identity.

## Safety boundary

Subscription client identity handling is user-owned state only.

It must not:

- require root;
- start `tunwardend`;
- start Xray;
- create TUN devices;
- mutate routes;
- mutate DNS;
- mutate nftables or firewall state;
- write generated runtime core configuration;
- store the identity in `/etc`, the repository, daemon-private state, or generated runtime directories.

Identity placeholder resolution must stay in the shared HTTP(S) subscription request-construction path used by both `tunwarden import <http-url>` and `tunwarden subscription update`. Command handlers must not duplicate request identity logic.
