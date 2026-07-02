package check

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	// SchemaVersion is the stable JSON schema version used by profile checks.
	SchemaVersion = "v1"

	// DefaultTimeout bounds each individual network probe.
	DefaultTimeout = 3 * time.Second

	// DefaultBatchConcurrency is intentionally conservative because temporary
	// proxy lifecycle is daemon-owned and exclusive in the current runtime model.
	DefaultBatchConcurrency = 1

	ProbeTypeHTTPS = "https"
)

// Target describes one documented low-impact service availability probe.
type Target struct {
	ID               string
	DisplayName      string
	Category         string
	ProbeType        string
	URL              string
	Timeout          time.Duration
	SuccessCondition string
	ProxyDNS         bool
	PrivacyNote      string
}

// Catalog returns the stable predefined target catalog.
func Catalog() []Target {
	out := append([]Target(nil), targetCatalog...)
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// DefaultTargetIDs returns the conservative target set used by `podlaz check`.
func DefaultTargetIDs() []string {
	return []string{"cloudflare"}
}

// GenericEgressTarget returns the target used for generic proxy egress checks.
func GenericEgressTarget() Target {
	target, ok := FindTarget("cloudflare")
	if !ok {
		panic("check target catalog is missing cloudflare")
	}
	return target
}

// FindTarget returns a target by its stable id.
func FindTarget(id string) (Target, bool) {
	id = strings.ToLower(strings.TrimSpace(id))
	for _, target := range targetCatalog {
		if target.ID == id {
			return target, true
		}
	}
	return Target{}, false
}

// SelectTargets validates requested target ids and returns deterministic targets.
// Empty input selects the conservative default target set.
func SelectTargets(ids []string) ([]Target, error) {
	if len(ids) == 0 {
		ids = DefaultTargetIDs()
	}

	seen := make(map[string]struct{}, len(ids))
	out := make([]Target, 0, len(ids))
	for _, rawID := range ids {
		id := strings.ToLower(strings.TrimSpace(rawID))
		if id == "" {
			return nil, fmt.Errorf("check target id cannot be empty")
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		target, ok := FindTarget(id)
		if !ok {
			return nil, fmt.Errorf("unknown check target %q (supported: %s)", rawID, strings.Join(SupportedTargetIDs(), ", "))
		}
		seen[id] = struct{}{}
		out = append(out, target)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// SupportedTargetIDs returns all known target ids in deterministic order.
func SupportedTargetIDs() []string {
	targets := Catalog()
	ids := make([]string, len(targets))
	for i, target := range targets {
		ids[i] = target.ID
	}
	return ids
}

var targetCatalog = []Target{
	{
		ID:               "cloudflare",
		DisplayName:      "Cloudflare",
		Category:         "generic-egress",
		ProbeType:        ProbeTypeHTTPS,
		URL:              "https://" + "www.cloudflare.com/cdn-cgi/trace",
		Timeout:          DefaultTimeout,
		SuccessCondition: "HTTP status below 500",
		ProxyDNS:         true,
		PrivacyNote:      "Cloudflare receives a single HTTPS request from the selected proxy path.",
	},
	{
		ID:               "github",
		DisplayName:      "GitHub",
		Category:         "developer-service",
		ProbeType:        ProbeTypeHTTPS,
		URL:              "https://" + "github.com/favicon.ico",
		Timeout:          DefaultTimeout,
		SuccessCondition: "HTTP status below 500",
		ProxyDNS:         true,
		PrivacyNote:      "GitHub receives a single HTTPS request from the selected proxy path.",
	},
	{
		ID:               "google",
		DisplayName:      "Google",
		Category:         "search-service",
		ProbeType:        ProbeTypeHTTPS,
		URL:              "https://" + "www.google.com/generate_204",
		Timeout:          DefaultTimeout,
		SuccessCondition: "HTTP status 204 or any non-server-error status",
		ProxyDNS:         true,
		PrivacyNote:      "Google receives a single HTTPS connectivity-check request from the selected proxy path.",
	},
	{
		ID:               "telegram",
		DisplayName:      "Telegram",
		Category:         "messaging-service",
		ProbeType:        ProbeTypeHTTPS,
		URL:              "https://" + "api.telegram.org/",
		Timeout:          DefaultTimeout,
		SuccessCondition: "TLS and HTTP response with status below 500",
		ProxyDNS:         true,
		PrivacyNote:      "Telegram receives a single HTTPS request from the selected proxy path.",
	},
}
