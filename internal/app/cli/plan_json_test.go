package cli

import (
	"testing"

	"github.com/AidarKhusainov/podlaz/internal/network/planner"
)

func TestProxyOnlyPlanJSONDocumentsReadOnlyEffects(t *testing.T) {
	got := proxyOnlyPlanJSON(planner.ProxyOnlyPlan{
		Mode:              planner.ModeProxyOnly,
		ProfileID:         "profile-1",
		ProfileName:       "demo",
		RuntimeConfigPath: "/run/podlaz/generated/xray.json",
		Listeners: []planner.Listener{
			{Protocol: "SOCKS", Address: "127.0.0.1", Port: 1080},
			{Protocol: "HTTP", Address: "127.0.0.1", Port: 8080},
		},
	})

	if got["schema_version"] != "v1" || got["status"] != "ok" || got["mode"] != planner.ModeProxyOnly {
		t.Fatalf("unexpected top-level JSON fields: %#v", got)
	}
	plan, ok := got["plan"].(map[string]any)
	if !ok {
		t.Fatalf("plan field has unexpected type: %#v", got["plan"])
	}
	for field, want := range map[string]bool{
		"writes_config":                false,
		"starts_xray":                  false,
		"modifies_system_networking":   false,
	} {
		if plan[field] != want {
			t.Fatalf("expected plan[%q] to be %v, got %#v", field, want, plan[field])
		}
	}
	listeners, ok := plan["listeners"].([]map[string]any)
	if !ok || len(listeners) != 2 {
		t.Fatalf("expected two listeners, got %#v", plan["listeners"])
	}
	if listeners[0]["protocol"] != "socks" || listeners[0]["address"] != "127.0.0.1" || listeners[0]["port"] != uint16(1080) {
		t.Fatalf("unexpected SOCKS listener JSON: %#v", listeners[0])
	}
	if listeners[1]["protocol"] != "http" || listeners[1]["address"] != "127.0.0.1" || listeners[1]["port"] != uint16(8080) {
		t.Fatalf("unexpected HTTP listener JSON: %#v", listeners[1])
	}
}
