package sub

import (
	"encoding/json"
	"strings"

	"github.com/AidarKhusainov/podlaz/internal/profile"
)

func looksLikeGroupedProviderXrayObject(content []byte) bool {
	object, err := decodeSubscriptionJSONObject(content)
	if err != nil {
		return false
	}
	if _, hasRouting := object["routing"]; !hasRouting {
		return false
	}
	rawOutbounds, ok := object["outbounds"]
	if !ok {
		return false
	}
	var outbounds []struct {
		Protocol string `json:"protocol"`
	}
	if err := json.Unmarshal(rawOutbounds, &outbounds); err != nil {
		return false
	}
	vlessOutbounds := 0
	for _, outbound := range outbounds {
		if strings.EqualFold(strings.TrimSpace(outbound.Protocol), "vless") {
			vlessOutbounds++
		}
	}
	return vlessOutbounds >= 2
}

func parseGroupedProviderXrayProfile(content []byte) (Parsed, error) {
	name, ok := xrayJSONWrapperProfileDisplayName(content)
	if !ok {
		name = "Xray JSON grouped profile"
	}
	p, acceptedName, err := profile.NewSubscriptionProviderXrayConfig(name, content)
	if err != nil {
		return Parsed{}, err
	}
	parsed := Parsed{Profiles: []profile.Profile{p}}
	if !acceptedName {
		parsed.Warnings = append(parsed.Warnings, Issue{Line: 1, Message: profile.DisplayNameRejectedWarning})
	}
	return parsed, nil
}
