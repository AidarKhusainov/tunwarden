package sub

import "testing"

func TestMetadataDisplayNameUsesWrapperRemark(t *testing.T) {
	body := []byte(`{"remark":"Synthetic Nodes"}`)
	name, warnings := ProviderSubscriptionDisplayNameFromMetadata(FormatXrayJSON, body, nil)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if name != "Synthetic Nodes" {
		t.Fatalf("expected wrapper display name, got %q", name)
	}
}
