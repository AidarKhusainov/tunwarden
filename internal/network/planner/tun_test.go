package planner

import (
	"strings"
	"testing"

	"github.com/AidarKhusainov/tunwarden/internal/network/snapshot"
)

func TestPlanTunBuildsFullTunnelPlanFromFakeSnapshot(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeResolvedDesktop())
	if err != nil {
		t.Fatalf("plan tun: %v", err)
	}

	if plan.Mode != ModeTun || plan.TunnelMode != TunTunnelMode {
		t.Fatalf("unexpected TUN mode: mode=%q tunnel=%q", plan.Mode, plan.TunnelMode)
	}
	if plan.TunDevice.Name != snapshot.DefaultTunName || plan.TunDevice.MTU != DefaultTunMTU || plan.TunDevice.Action != "create" {
		t.Fatalf("unexpected TUN device plan: %#v", plan.TunDevice)
	}
	if !containsRoute(plan.Routes, TunRoutingTable, IPv4DefaultRoute, snapshot.DefaultTunName) {
		t.Fatalf("expected default route through %s table, got %#v", TunRoutingTable, plan.Routes)
	}
	if plan.ServerBypass.Destination != "203.0.113.10/32" || plan.ServerBypass.Gateway != "192.0.2.1" || plan.ServerBypass.Interface != "wlp0s20f3" || plan.ServerBypass.Table != MainRoutingTable {
		t.Fatalf("unexpected server bypass route: %#v", plan.ServerBypass)
	}
	if !containsPolicyRule(plan.PolicyRules, ServerRulePriority, "to 203.0.113.10/32", MainRoutingTable) {
		t.Fatalf("expected server bypass policy rule, got %#v", plan.PolicyRules)
	}
	if !containsPolicyRule(plan.PolicyRules, TunRulePriority, IPv4DefaultSelector, TunRoutingTable) {
		t.Fatalf("expected full-tunnel policy rule, got %#v", plan.PolicyRules)
	}
	if len(plan.RollbackSteps) == 0 {
		t.Fatalf("expected rollback steps for planned route/TUN changes")
	}
	if len(plan.LoopRisks) != 0 {
		t.Fatalf("expected clean fake snapshot to have no loop risk, got %#v", plan.LoopRisks)
	}
}

func TestPlanTunWarnsWhenDefaultRouteIsMissing(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeDesktopMissingDefaultRoute())
	if err != nil {
		t.Fatalf("plan tun with missing default route: %v", err)
	}
	if !containsWarning(plan.Warnings, "IPv4 default route is missing") {
		t.Fatalf("expected missing default route warning, got %#v", plan.Warnings)
	}
	if !containsWarning(plan.Warnings, "VPN server bypass target") {
		t.Fatalf("expected incomplete server bypass warning, got %#v", plan.Warnings)
	}
}

func TestPlanTunWarnsAboutExistingTunWardenTun(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeDesktopWithStaleTunWardenResources())
	if err != nil {
		t.Fatalf("plan tun with existing TunWarden resources: %v", err)
	}
	if !containsWarning(plan.Warnings, "TunWarden TUN device tunwarden0 already exists") {
		t.Fatalf("expected existing TUN warning, got %#v", plan.Warnings)
	}
	if !containsWarning(plan.Warnings, "stale TunWarden-owned") {
		t.Fatalf("expected stale-resource warning, got %#v", plan.Warnings)
	}
}

func TestPlanTunDetectsServerRouteLoopRisk(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeDesktopWithServerRouteLoop())
	if err != nil {
		t.Fatalf("plan tun with route loop: %v", err)
	}
	if len(plan.LoopRisks) == 0 {
		t.Fatalf("expected route loop risks")
	}
	if !containsWarning(plan.Warnings, "route to VPN server candidate points at tunwarden0") {
		t.Fatalf("expected route loop warning, got %#v", plan.Warnings)
	}
}

func TestPlanTunWarnsAboutMissingOptionalTools(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeDesktopWithoutOptionalTools())
	if err != nil {
		t.Fatalf("plan tun without optional tools: %v", err)
	}
	for _, want := range []string{"systemd-resolved", "nftables"} {
		if !containsWarning(plan.Warnings, want) {
			t.Fatalf("expected warning containing %q, got %#v", want, plan.Warnings)
		}
	}
}

func containsRoute(routes []TunRoutePlan, table, destination, iface string) bool {
	for _, route := range routes {
		if route.Table == table && route.Destination == destination && route.Interface == iface {
			return true
		}
	}
	return false
}

func containsPolicyRule(rules []TunPolicyRulePlan, priority int, selector, table string) bool {
	for _, rule := range rules {
		if rule.Priority == priority && rule.Selector == selector && rule.Table == table {
			return true
		}
	}
	return false
}

func containsWarning(warnings []string, want string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, want) {
			return true
		}
	}
	return false
}
