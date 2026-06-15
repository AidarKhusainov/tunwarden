# Subscriptions and Profiles

## 1. Purpose

TunWarden must support both direct/manual connections and subscription-based profiles.

The goal is not to preserve every provider-specific detail forever. The goal is to normalize different inputs into a stable internal profile model that can be validated, tested, and converted into runtime core configuration.

Implemented manual profile management behavior is documented in [Profile management](./profile-management.md).

## 2. Input sources

### 2.1 Manual profiles

Manual profiles should be supported from the beginning.

Initial protocols to consider:

- VLESS,
- VMess,
- Trojan,
- Shadowsocks.

Manual profiles are required for development because they make networking tests independent from subscription providers.

The v0.1 foundation implementation supports explicit manual `profile add`, `profile list`, `profile show`, and `profile delete --yes` commands for persistent user-owned local profile state.

### 2.2 Subscription URLs

TunWarden must support adding subscription URLs.

HTTP(S) subscription fetches must send an explicit `User-Agent: TunWarden` request header. The value intentionally identifies TunWarden without pretending to be a browser or another VPN/proxy client, and it must not include provider tokens, user identities, operating-system details, device details, or other fine-grained fingerprinting data.

Subscription client identity/HWID behavior is owned by [Subscription client identity](./subscription-client-identity.md). Until a provider-specific Remnawave/Happ wire contract is confirmed with sanitized evidence, HTTP(S) subscription fetches must not add guessed HWID/device query parameters or headers. They must not send raw `/etc/machine-id`, MAC addresses, hostnames, DMI serials, disk serials, CPU identifiers, or other raw hardware identifiers.

Initial command shape:

```bash
tunwarden subscription add personal https://example.com/sub
tunwarden subscription update personal
tunwarden subscription list
tunwarden subscription remove personal
```

### 2.3 Imported files

Future support:

- local JSON files,
- local YAML files,
- exported Xray configs,
- sing-box configs,
- Mihomo/Clash YAML.

## 3. Subscription format families

TunWarden should be designed around format adapters.

```text
SubscriptionSource
  -> Fetcher
  -> FormatDetector
  -> Parser
  -> Normalizer
  -> Validator
  -> ProfileStore
```

Expected format families:

- Base64 list of share links,
- plain text share links,
- Xray JSON,
- sing-box JSON,
- Mihomo/Clash YAML,
- provider-specific templates such as Remnawave,
- 3x-ui compatible subscription outputs.

## 4. Share link support

Initial URI schemes:

```text
vless://
vmess://
trojan://
ss://
```

Future URI schemes:

```text
hysteria://
hysteria2://
tuic://
wireguard://
```

Unsupported URI schemes must produce clear errors, not silent skips.

## 5. Internal profile model

Every imported node must be normalized to an internal model.

Suggested fields:

```text
Profile
  id
  name
  source
  protocol
  server
  port
  user_identity
  security
  transport
  mux
  packet_encoding
  udp_support
  dns_policy
  routing_policy
  tags
  provider_metadata
  raw_source_reference
  created_at
  updated_at
```

### 5.1 Source metadata

```text
ProfileSource
  type: manual | subscription | imported_file
  subscription_id
  provider_name
  original_url
  original_format
  last_updated_at
```

### 5.2 Security model

Security fields must not be flattened into unstructured strings.

Examples:

```text
Security
  tls_enabled
  server_name
  alpn
  fingerprint
  reality
  allow_insecure
```

Reality-specific example:

```text
RealitySettings
  public_key
  short_id
  spider_x
```

### 5.3 Transport model

Examples:

```text
Transport
  type: tcp | ws | grpc | httpupgrade | xhttp | quic | kcp
  path
  host
  service_name
  headers
```

## 6. Validation requirements

### VAL-001: Required fields

Each normalized profile must validate required fields:

- protocol,
- server,
- port,
- protocol-specific identity/auth fields,
- transport compatibility,
- security compatibility.

### VAL-002: Unsafe settings warnings

TunWarden must warn about risky settings:

- `allowInsecure = true`,
- missing SNI when TLS requires it,
- unsupported transport,
- unsupported UDP behavior,
- unknown fingerprint,
- IPv6 enabled without full IPv6 routing support.

### VAL-003: Provider errors

Subscription update failures must preserve the last known good profiles unless the user explicitly removes them.

### VAL-004: Deterministic IDs

Imported profiles should receive deterministic IDs where possible so subscription updates do not duplicate nodes unnecessarily.

Candidate inputs:

```text
subscription_id + protocol + server + port + user_identity + transport + security fingerprint
```

## 7. Subscription update behavior

Subscription update must be safe.

Required behavior:

1. Fetch subscription.
2. Detect format.
3. Parse into candidate nodes.
4. Normalize.
5. Validate.
6. Produce update diff.
7. Apply only if parsing/validation is good enough.
8. Keep last good state if update fails.

Diff categories:

- added profiles,
- removed profiles,
- changed profiles,
- unchanged profiles,
- invalid profiles.

## 8. Profile selection

MVP selection can be manual.

Future selection features:

- latency test,
- URL test,
- auto-select fastest node,
- provider group selection,
- fallback group,
- rule-based group selection.

## 9. Runtime profile rendering

The internal profile model must be rendered into generated core config at connection time.

Important rule:

> Generated Xray config is runtime output, not the persistent source of truth.

This allows TunWarden to change routing/DNS/runtime behavior without rewriting imported subscription data.
