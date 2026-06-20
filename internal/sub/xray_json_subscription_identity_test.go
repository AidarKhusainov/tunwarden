package sub

import (
	"strings"
	"testing"
)

func TestParseXrayJSONSubscriptionKeepsDistinctWebSocketPaths(t *testing.T) {
	id := "00000000-0000-0000-0000-000000000399"
	first := xrayObjectWithTopLevelField(
		strings.Replace(remoteXrayConfigObject(id, "same-node.example", "proxy", "ws", "tls"), `"path": "/ws"`, `"path": "/one"`, 1),
		`"remarks":"first"`,
	)
	second := xrayObjectWithTopLevelField(
		strings.Replace(remoteXrayConfigObject(id, "same-node.example", "proxy", "ws", "tls"), `"path": "/ws"`, `"path": "/two"`, 1),
		`"remarks":"second"`,
	)

	_, parsed, err := ParseSubscriptionContent([]byte("[" + first + "," + second + "]"))
	if err != nil {
		t.Fatalf("distinct path entries should not collide: %v", err)
	}
	if len(parsed.Profiles) != 2 {
		t.Fatalf("expected two profiles, got %#v", parsed.Profiles)
	}
	if parsed.Profiles[0].ID == parsed.Profiles[1].ID {
		t.Fatalf("expected distinct profile IDs, got %#v", parsed.Profiles)
	}
}
