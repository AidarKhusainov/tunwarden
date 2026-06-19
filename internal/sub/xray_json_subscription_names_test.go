package sub

import "testing"

func TestParseXrayJSONSubscriptionUsesWrapperRemarksAsProfileName(t *testing.T) {
	body := xrayObjectWithTopLevelField(
		remoteXrayConfigObject("00000000-0000-0000-0000-000000000301", "remnawave-one.example", "proxy", "tcp", "tls"),
		`"remarks":"USA"`,
	)

	_, parsed, err := ParseSubscriptionContent([]byte(body))
	if err != nil {
		t.Fatalf("ParseSubscriptionContent failed: %v", err)
	}
	if len(parsed.Profiles) != 1 {
		t.Fatalf("expected one profile, got %#v", parsed.Profiles)
	}
	if parsed.Profiles[0].Name != "USA" {
		t.Fatalf("expected wrapper profile name, got %#v", parsed.Profiles[0])
	}
}
