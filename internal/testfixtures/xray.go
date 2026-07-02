package testfixtures

import "fmt"

const (
	GroupedXrayUserID          = "00000000-0000-4000-8000-000000000180"
	GroupedXraySecretToken     = "provider-secret-token"
	GroupedXrayRuntimeSentinel = "runtime-config-sentinel"
)

// GroupedProviderXrayJSON returns a synthetic, redacted, runtime-shaped grouped
// provider Xray config. It intentionally uses reserved .invalid hosts and a
// fake UUID while preserving realistic VLESS outbound, streamSettings, routing,
// balancer, and provider inbound structure.
func GroupedProviderXrayJSON() string {
	return `{
  "remarks": "Автоподбор локации",
  "log": {"loglevel": "warning"},
  "inbounds": [
    {
      "tag": "runtime-config-sentinel",
      "listen": "127.0.0.1",
      "port": 2080,
      "protocol": "socks",
      "settings": {"auth": "noauth", "udp": false}
    }
  ],
  "outbounds": [
    {
      "protocol": "vless",
      "tag": "auto",
      "settings": {
        "vnext": [{
          "address": "auto.edge.invalid",
          "port": 443,
          "users": [{"id": "00000000-0000-4000-8000-000000000180", "encryption": "none", "flow": "xtls-rprx-vision"}]
        }]
      },
      "streamSettings": {
        "network": "tcp",
        "security": "reality",
        "realitySettings": {
          "serverName": "auto.edge.invalid",
          "publicKey": "public-key-auto",
          "shortId": "a1b2",
          "spiderX": "/spider"
        }
      }
    },
    {
      "protocol": "vless",
      "tag": "ai",
      "settings": {
        "vnext": [{
          "address": "ai.edge.invalid",
          "port": 443,
          "users": [{"id": "00000000-0000-4000-8000-000000000180", "encryption": "none"}]
        }]
      },
      "streamSettings": {
        "network": "ws",
        "security": "tls",
        "tlsSettings": {"serverName": "ai.edge.invalid", "fingerprint": "chrome"},
        "wsSettings": {"path": "/ws", "headers": {"Host": "ai.edge.invalid", "X-Provider-Token": "provider-secret-token"}}
      }
    },
    {
      "protocol": "vless",
      "tag": "tg",
      "settings": {
        "vnext": [{
          "address": "tg.edge.invalid",
          "port": 443,
          "users": [{"id": "00000000-0000-4000-8000-000000000180", "encryption": "none"}]
        }]
      },
      "streamSettings": {
        "network": "xhttp",
        "security": "tls",
        "tlsSettings": {"serverName": "tg.edge.invalid", "fingerprint": "chrome"},
        "xhttpSettings": {"path": "/xhttp", "host": "tg.edge.invalid"}
      }
    }
  ],
  "routing": {
    "balancers": [{"tag": "auto", "selector": ["auto", "ai", "tg"]}],
    "rules": [
      {"type": "field", "domain": ["geosite:ai"], "outboundTag": "ai"},
      {"type": "field", "domain": ["geosite:telegram"], "outboundTag": "tg"},
      {"type": "field", "network": "tcp,udp", "balancerTag": "auto"}
    ]
  }
}`
}

// SingleVLESSXrayJSON returns one ordinary single-location Xray JSON object.
func SingleVLESSXrayJSON(tag, host, network, security string) string {
	return fmt.Sprintf(`{
  "remarks": %q,
  "outbounds": [
    {
      "protocol": "vless",
      "tag": %q,
      "settings": {
        "vnext": [{
          "address": %q,
          "port": 443,
          "users": [{"id": "00000000-0000-4000-8000-000000000180", "encryption": "none"}]
        }]
      },
      "streamSettings": {
        "network": %q,
        "security": %q,
        "tlsSettings": {"serverName": %q, "fingerprint": "chrome"},
        "realitySettings": {"serverName": %q, "publicKey": "public-key", "shortId": "abcd"},
        "wsSettings": {"path": "/ws", "headers": {"Host": %q}},
        "grpcSettings": {"serviceName": "svc"},
        "xhttpSettings": {"path": "/xhttp", "host": %q}
      }
    }
  ]
}`, tag, tag, host, network, security, host, host, host, host)
}
