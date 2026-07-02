package sub

import (
	"strings"
	"testing"

	"github.com/AidarKhusainov/podlaz/internal/profile"
)

func TestParseXrayJSONObjectSubscriptionImportsGroupedProviderProfile(t *testing.T) {
	parsed, err := ParseXrayJSONSubscription([]byte(`{
		"remarks": "Grouped provider",
		"outbounds": [{"protocol": "vless"}, {"protocol": "vless"}],
		"routing": {"rules": [{"type": "field", "balancerTag": "auto"}]}
	}`))
	if err != nil {
		t.Fatalf("ParseXrayJSONSubscription failed: %v", err)
	}
	if got, want := len(parsed.Profiles), 1; got != want {
		t.Fatalf("expected %d profile, got %d", want, got)
	}
	p := parsed.Profiles[0]
	if p.Protocol != profile.ProtocolXrayJSON {
		t.Fatalf("expected grouped protocol %q, got %q", profile.ProtocolXrayJSON, p.Protocol)
	}
	if p.Server != "" || p.Port != 0 {
		t.Fatalf("expected grouped profile not to collapse to one endpoint, got server=%q port=%d", p.Server, p.Port)
	}
	if strings.TrimSpace(profile.ProviderXrayConfigJSON(p)) == "" {
		t.Fatal("expected grouped provider config to be stored")
	}
}
