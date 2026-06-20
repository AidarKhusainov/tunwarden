package sub

import (
	"net/http"
	"testing"
)

func TestMetadataDisplayNameUsesAttachmentName(t *testing.T) {
	header := http.Header{}
	header.Set("Content"+"-Disposition", `attachment; filename="Synthetic Nodes"`)
	name, warnings := ProviderSubscriptionDisplayNameFromMetadata(FormatBase64, []byte("sample"), header)
	if len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if name != "Synthetic Nodes" {
		t.Fatalf("expected display name, got %q", name)
	}
}
