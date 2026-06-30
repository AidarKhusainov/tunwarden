package planner

import (
	"reflect"
	"testing"

	"github.com/AidarKhusainov/podlaz/internal/network/snapshot"
)

func TestPlanTunPinsCurrentDefaultPolicyShape(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeResolvedDesktop())
	if err != nil {
		t.Fatalf("plan tun: %v", err)
	}

	if plan.Mode != ModeTun || plan.TunnelMode != TunTunnelMode {
		t.Fatalf("unexpected TUN policy mode: mode=%q tunnel=%q", plan.Mode, plan.TunnelMode)
	}
	if plan.TunDevice.Name != snapshot.DefaultTunName || plan.TunDevice.Action != "create" || plan.TunDevice.MTU != DefaultTunMTU {
		t.Fatalf("unexpected TUN device policy: %#v", plan.TunDevice)
	}
	if got, want := len(plan.Routes), 2; got != want {
		t.Fatalf("route count = %d, want %d: %#v", got, want, plan.Routes)
	}
	if !containsRoute(plan.Routes, TunRoutingTable, IPv4DefaultRoute, snapshot.DefaultTunName) {
		t.Fatalf("missing full-tunnel default route: %#v", plan.Routes)
	}
	if plan.ServerBypass.Destination != "203.0.113.10/32" || plan.ServerBypass.Table != MainRoutingTable || plan.ServerBypass.Action != "add" {
		t.Fatalf("unexpected server bypass route: %#v", plan.ServerBypass)
	}
	if got, want := len(plan.PolicyRules), 2; got != want {
		t.Fatalf("policy rule count = %d, want %d: %#v", got, want, plan.PolicyRules)
	}
	if !containsPolicyRule(plan.PolicyRules, ServerRulePriority, "to 203.0.113.10/32", MainRoutingTable) {
		t.Fatalf("missing server-bypass policy rule: %#v", plan.PolicyRules)
	}
	if !containsPolicyRule(plan.PolicyRules, TunRulePriority, IPv4DefaultSelector, TunRoutingTable) {
		t.Fatalf("missing full-tunnel policy rule: %#v", plan.PolicyRules)
	}
	if !reflect.DeepEqual(plan.DNS.Servers, []string{DefaultTunDNSServer}) {
		t.Fatalf("DNS servers = %#v, want default %q", plan.DNS.Servers, DefaultTunDNSServer)
	}
	if plan.Firewall.KillSwitch.Policy != KillSwitchPolicySoft {
		t.Fatalf("default kill-switch policy = %#v, want %q", plan.Firewall.KillSwitch, KillSwitchPolicySoft)
	}
	if !containsFirewallRule(plan.Firewall.Rules, FirewallKillSwitchOwner, FirewallActionAdd, FirewallVerdictReject, `oifname != "podlaz0"`) {
		t.Fatalf("missing default soft kill-switch rule: %#v", plan.Firewall.Rules)
	}
}

func TestPlanTunPinsWarningBehaviorForRiskyInputs(t *testing.T) {
	tests := []struct {
		name     string
		plan     func(t *testing.T) TunPlan
		warnings []string
	}{
		{
			name: "missing default route",
			plan: func(t *testing.T) TunPlan {
				plan, err := PlanTun(testVLESSProfile(), snapshot.FakeDesktopMissingDefaultRoute())
				if err != nil {
					t.Fatalf("plan tun: %v", err)
				}
				return plan
			},
			warnings: []string{
				"IPv4 default route is missing",
				"route to VPN server candidate is missing",
				"VPN server bypass target is unknown",
			},
		},
		{
			name: "strict kill switch",
			plan: func(t *testing.T) TunPlan {
				plan, err := PlanTunWithOptions(testVLESSProfile(), snapshot.FakeResolvedDesktop(), TunOptions{KillSwitchPolicy: KillSwitchPolicyStrict})
				if err != nil {
					t.Fatalf("plan tun: %v", err)
				}
				return plan
			},
			warnings: []string{
				"strict kill-switch policy selected",
				"strict kill-switch may intentionally keep direct connectivity blocked",
				"strict kill-switch cannot be claimed as leak protection",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := tt.plan(t)
			for _, warning := range tt.warnings {
				if !containsWarning(plan.Warnings, warning) {
					t.Fatalf("expected warning containing %q, got %#v", warning, plan.Warnings)
				}
			}
		})
	}
}

func TestPlanTunPinsRollbackStepOrderingAndDomains(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeResolvedDesktop())
	if err != nil {
		t.Fatalf("plan tun: %v", err)
	}

	wantPrefix := []string{
		"Remove nftables table inet podlaz if created by this transaction",
		"Restore previous systemd-resolved per-link DNS state for podlaz0 where possible",
		"Flush podlaz-owned DNS server/default-route/domain settings from podlaz0 if created by this transaction",
		"Delete policy rule priority 10000 from all lookup podlaz if created by this transaction",
		"Delete policy rule priority 9999 to 203.0.113.10/32 lookup main if created by this transaction",
	}
	if len(plan.RollbackSteps) < len(wantPrefix) {
		t.Fatalf("rollback step count = %d, want at least %d: %#v", len(plan.RollbackSteps), len(wantPrefix), plan.RollbackSteps)
	}
	for i, want := range wantPrefix {
		if plan.RollbackSteps[i] != want {
			t.Fatalf("rollback step %d = %q, want %q; all steps: %#v", i, plan.RollbackSteps[i], want, plan.RollbackSteps)
		}
	}
	for _, want := range []string{
		"Delete route 203.0.113.10/32 from table main via 192.0.2.1 dev wlp0s20f3 if created by this transaction",
		"Delete route default from table podlaz dev podlaz0 if created by this transaction",
		"Delete TUN interface podlaz0 only if this transaction created it and ownership matches podlaz",
	} {
		if !containsString(plan.RollbackSteps, want) {
			t.Fatalf("expected rollback step %q, got %#v", want, plan.RollbackSteps)
		}
	}
}
