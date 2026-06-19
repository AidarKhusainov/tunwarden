package sub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/AidarKhusainov/podlaz/internal/profile"
)

const SubscriptionDisplayNameRejectedWarning = "provider subscription display name was rejected; using safe fallback"

// ProviderSubscriptionDisplayName extracts a subscription-level display name from
// known, tested provider metadata fields. It intentionally ignores entry-level
// profile names and provider error/unsupported-client messages.
func ProviderSubscriptionDisplayName(format Format, content []byte) (string, []Issue) {
	return ProviderSubscriptionDisplayNameFromMetadata(format, content, nil)
}

// ProviderSubscriptionDisplayNameFromMetadata extracts a subscription-level
// display name from safe response metadata first, then from known JSON wrapper
// metadata fields. It never uses raw URLs or query parameters as provider names.
func ProviderSubscriptionDisplayNameFromMetadata(format Format, content []byte, header http.Header) (string, []Issue) {
	if name, warnings, ok := firstSafeSubscriptionDisplayName(subscriptionHeaderDisplayNameCandidates(header)); ok {
		return name, warnings
	}
	if format != FormatXrayJSON {
		return "", nil
	}
	trimmed := bytes.TrimSpace(content)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return "", nil
	}

	object, err := decodeSubscriptionJSONObject(trimmed)
	if err != nil {
		return "", nil
	}
	if _, unsupported := unsupportedClientProviderMessage(object); unsupported {
		return "", nil
	}

	for _, key := range []string{"title", "name", "remarks", "remark"} {
		raw, ok := object[key]
		if !ok {
			continue
		}
		value, ok := rawJSONString(raw)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		return sanitizeSubscriptionDisplayName(value)
	}
	return "", nil
}

func subscriptionHeaderDisplayNameCandidates(header http.Header) []string {
	if len(header) == 0 {
		return nil
	}
	var candidates []string
	if disposition := header.Get("Content-Disposition"); strings.TrimSpace(disposition) != "" {
		if _, params, err := mime.ParseMediaType(disposition); err == nil {
			for _, key := range []string{"filename", "name"} {
				if value := strings.TrimSpace(params[key]); value != "" {
					candidates = append(candidates, value)
				}
			}
		}
	}
	for _, key := range []string{
		"Subscription-Title",
		"Profile-Title",
		"X-Subscription-Title",
		"X-Profile-Title",
		"Subscription-Name",
		"Profile-Name",
		"X-Subscription-Name",
		"X-Profile-Name",
	} {
		for _, value := range header.Values(key) {
			if strings.TrimSpace(value) != "" {
				candidates = append(candidates, value)
			}
		}
	}
	return candidates
}

func firstSafeSubscriptionDisplayName(candidates []string) (string, []Issue, bool) {
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		name, warnings := sanitizeSubscriptionDisplayName(candidate)
		return name, warnings, true
	}
	return "", nil, false
}

func sanitizeSubscriptionDisplayName(raw string) (string, []Issue) {
	if name, ok := profile.SanitizeDisplayName(raw); ok {
		return name, nil
	}
	return "", []Issue{{Line: 1, Message: SubscriptionDisplayNameRejectedWarning}}
}

func decodeSubscriptionJSONObject(content []byte) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	var object map[string]json.RawMessage
	if err := decoder.Decode(&object); err != nil {
		return nil, fmt.Errorf("malformed Xray JSON subscription object: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return nil, fmt.Errorf("malformed Xray JSON subscription object: trailing data")
	}
	return object, nil
}

// RefreshProviderDisplayName updates provider-owned subscription display names
// while keeping the subscription ID as the durable command-facing identity.
func RefreshProviderDisplayName(source Source, providerName string) Source {
	if strings.TrimSpace(providerName) == "" {
		return source
	}
	if source.ID != profile.NormalizeID(fallbackSubscriptionDisplayName(source.URL)) {
		return source
	}
	name, ok := profile.SanitizeDisplayName(providerName)
	if !ok {
		return source
	}
	source.Name = name
	return source
}
