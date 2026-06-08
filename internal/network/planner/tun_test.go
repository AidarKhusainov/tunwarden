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
	if plan.DNS.Backend != DNSBackendSystemdResolved || plan.DNS.TargetLink != snapshot.DefaultTunName || plan.DNS.Action != DNSActionConfigure {
		t.Fatalf("unexpected DNS plan: %#v", plan.DNS)
	}
	if plan.Firewall.Backend != FirewallBackendNftables || plan.Firewall.Family != snapshot.DefaultNFTFamily || plan.Firewall.Table != snapshot.DefaultNFTTable || plan.Firewall.TableAction != FirewallTableAction {
		t.Fatalf("unexpected firewall plan: %#v", plan.Firewall)
	}
	if plan.Firewall.KillSwitch.Policy != KillSwitchPolicySoft || !strings.Contains(plan.Firewall.KillSwitch.Action, "block non-TUN traffic") {
		t.Fatalf("unexpected kill-switch plan: %#v", plan.Firewall.KillSwitch)
	}
	if len(plan.RollbackSteps) == 0 {
		t.Fatalf("expected rollback steps for planned route/TUN changes")
	}
	if !containsString(plan.RollbackSteps, "Restore previous systemd-resolved") || !containsString(plan.RollbackSteps, "Remove nftables table inet tunwarden") {
		t.Fatalf("expected DNS and nftables rollback steps, got %#v", plan.RollbackSteps)
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
	if plan.Firewall.TableAction != "validate-or-replace" {
		t.Fatalf("expected existing nftables table to require validation/replacement, got %#v", plan.Firewall)
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

func TestPlanTunPlansResolvedDNSWhenPresent(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeResolvedDesktop())
	if err != nil {
		t.Fatalf("plan tun with resolved present: %v", err)
	}
	if plan.DNS.Action != DNSActionConfigure || plan.DNS.Backend != DNSBackendSystemdResolved || plan.DNS.TargetLink != snapshot.DefaultTunName {
		t.Fatalf("expected systemd-resolved DNS desired state, got %#v", plan.DNS)
	}
}

func TestPlanTunBlocksDNSPlanWhenResolvedMissing(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), fakeDesktopWithoutResolved())
	if err != nil {
		t.Fatalf("plan tun with resolved missing: %v", err)
	}
	if plan.DNS.Action != DNSActionBlocked {
		t.Fatalf("expected DNS plan to be blocked, got %#v", plan.DNS)
	}
	if !containsWarning(plan.Warnings, "DNS desired state is blocked") {
		t.Fatalf("expected blocked DNS warning, got %#v", plan.Warnings)
	}
}

func TestPlanTunPlansNftablesWhenPresent(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeResolvedDesktop())
	if err != nil {
		t.Fatalf("plan tun with nftables present: %v", err)
	}
	if plan.Firewall.TableAction != FirewallTableAction || plan.Firewall.ServerBypass != "allow VPN server bypass outside TUN" {
		t.Fatalf("expected nftables firewall desired state, got %#v", plan.Firewall)
	}
}

func TestPlanTunBlocksFirewallPlanWhenNftablesMissing(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), fakeDesktopWithoutNftables())
	if err != nil {
		t.Fatalf("plan tun with nftables missing: %v", err)
	}
	if plan.Firewall.TableAction != FirewallActionBlocked {
		t.Fatalf("expected firewall plan to be blocked, got %#v", plan.Firewall)
	}
	if !containsWarning(plan.Warnings, "firewall desired state is blocked") {
		t.Fatalf("expected blocked firewall warning, got %#v", plan.Warnings)
	}
}

func TestPlanTunWarnsAboutStrictKillSwitchPolicy(t *testing.T) {
	plan, err := PlanTunWithOptions(testVLESSProfile(), snapshot.FakeResolvedDesktop(), TunOptions{KillSwitchPolicy: KillSwitchPolicyStrict})
	if err != nil {
		t.Fatalf("plan tun with strict kill-switch: %v", err)
	}
	if plan.Firewall.KillSwitch.Policy != KillSwitchPolicyStrict {
		t.Fatalf("expected strict kill-switch policy, got %#v", plan.Firewall.KillSwitch)
	}
	if !containsWarning(plan.Warnings, "strict kill-switch") || !containsWarning(plan.Warnings, "recover") {
		t.Fatalf("expected strict kill-switch recovery warning, got %#v", plan.Warnings)
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
	return containsString(warnings, want)
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if strings.Contains(value, want) {
			return true
		}
	}
	return false
}

func fakeDesktopWithoutResolved() snapshot.Snapshot {
	s := snapshot.FakeResolvedDesktop()
	s.DNS = snapshot.DNS{Mode: "unknown", Resolved: snapshot.Finding{Status: snapshot.StatusMissing, Summary: "resolvectl not found"}}
	return s
}

func fakeDesktopWithoutNftables() snapshot.Snapshot {
	s := snapshot.FakeResolvedDesktop()
	s.Nftables = snapshot.Nftables{
		Availability:   snapshot.Finding{Status: snapshot.StatusMissing, Summary: "nft not found"},
		TunWardenTable: snapshot.Finding{Status: snapshot.StatusMissing, Summary: "TunWarden nftables table not inspected because nft is unavailable"},
	}
	return s
}
