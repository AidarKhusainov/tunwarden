package planner

import (
	"fmt"
	"net"
	"strings"

	"github.com/AidarKhusainov/tunwarden/internal/network/snapshot"
	"github.com/AidarKhusainov/tunwarden/internal/profile"
)

const (
	ModeTun = "tun"

	TunTunnelMode       = "full-tunnel"
	DefaultTunMTU       = 1500
	TunRoutingTable     = "tunwarden"
	TunRoutingTableID   = 51820
	TunRulePriority     = 10000
	ServerRulePriority  = 9999
	MainRoutingTable    = "main"
	IPv4DefaultRoute    = "default"
	IPv4DefaultSelector = "from all"

	DNSBackendSystemdResolved = "systemd-resolved per-link DNS"
	DNSActionConfigure        = "configure"
	DNSActionBlocked          = "blocked"
	DNSRollbackRestore        = "restore previous per-link DNS state where possible"
	DefaultTunDNSServer       = "1.1.1.1"

	FirewallBackendNftables    = "nftables"
	FirewallTableAction        = "create"
	FirewallActionAdd          = "add"
	FirewallActionBlocked      = "blocked"
	FirewallActionSkip         = "skip"
	FirewallActionValidate     = "validate-or-replace"
	FirewallRollbackRemove     = "remove inet tunwarden"
	FirewallOutputChain        = "output"
	FirewallChainTypeFilter    = "filter"
	FirewallOutputHook         = "output"
	FirewallOutputPriority     = 0
	FirewallDefaultChainPolicy = "accept"
	FirewallVerdictAccept      = "accept"
	FirewallVerdictReject      = "reject"
	FirewallVerdictDrop        = "drop"
	FirewallServerBypassOwner  = "tunwarden:firewall:server-bypass"
	FirewallTunEgressOwner     = "tunwarden:firewall:tun-egress"
	FirewallKillSwitchOwner    = "tunwarden:firewall:kill-switch"
	FirewallServerBypassKey    = "inet/tunwarden/output/server-bypass"
	FirewallTunEgressKey       = "inet/tunwarden/output/tun-egress"
	FirewallKillSwitchKey      = "inet/tunwarden/output/kill-switch"

	KillSwitchPolicyOff    = "off"
	KillSwitchPolicySoft   = "soft"
	KillSwitchPolicyStrict = "strict"
)

type TunOptions struct {
	KillSwitchPolicy string
	DNSServers       []string
}

type TunDevicePlan struct {
	Name   string
	MTU    int
	Action string
	Reason string
}

type TunRoutePlan struct {
	Family      string
	Destination string
	Table       string
	Interface   string
	Gateway     string
	Action      string
	Reason      string
}

type TunPolicyRulePlan struct {
	Family   string
	Priority int
	Selector string
	Table    string
	Action   string
	Reason   string
}

type TunDNSPlan struct {
	Backend       string
	TargetLink    string
	Servers       []string
	Action        string
	Reason        string
	Rollback      string
	RollbackSteps []string
}

type TunFirewallPlan struct {
	Backend       string
	Family        string
	Table         string
	TableAction   string
	Chains        []TunFirewallChainPlan
	Rules         []TunFirewallRulePlan
	KillSwitch    TunKillSwitchPlan
	Reason        string
	Rollback      string
	RollbackSteps []string
}

type TunFirewallChainPlan struct {
	Name     string
	Type     string
	Hook     string
	Priority int
	Policy   string
	Action   string
	Reason   string
}

type TunFirewallRulePlan struct {
	Chain       string
	Expr        string
	Verdict     string
	Action      string
	Reason      string
	Ownership   string
	RollbackKey string
}

type TunKillSwitchPlan struct {
	Policy      string
	Action      string
	Scope       string
	Recovery    string
	Limitations []string
}

type TunPlan struct {
	Mode          string
	TunnelMode    string
	ProfileID     string
	ProfileName   string
	Snapshot      snapshot.Snapshot
	TunDevice     TunDevicePlan
	Routes        []TunRoutePlan
	PolicyRules   []TunPolicyRulePlan
	ServerBypass  TunRoutePlan
	DNS           TunDNSPlan
	Firewall      TunFirewallPlan
	LoopRisks     []string
	Warnings      []string
	Steps         []string
	RollbackSteps []string
}

func PlanTun(p profile.Profile, s snapshot.Snapshot) (TunPlan, error) {
	return PlanTunWithOptions(p, s, TunOptions{})
}

func PlanTunWithOptions(p profile.Profile, s snapshot.Snapshot, opts TunOptions) (TunPlan, error) {
	if err := profile.Validate(p); err != nil {
		return TunPlan{}, err
	}

	device := TunDevicePlan{Name: snapshot.DefaultTunName, MTU: DefaultTunMTU, Action: "create", Reason: "stable TunWarden full-tunnel interface"}
	serverIP := concreteServerBypassIP(s)
	serverBypass := serverBypassRoute(s, serverIP)
	routes := []TunRoutePlan{{
		Family:      "ipv4",
		Destination: IPv4DefaultRoute,
		Table:       TunRoutingTable,
		Interface:   snapshot.DefaultTunName,
		Action:      "add",
		Reason:      "route default IPv4 traffic through the TunWarden TUN interface",
	}}
	policyRules := []TunPolicyRulePlan{{
		Family:   "ipv4",
		Priority: TunRulePriority,
		Selector: IPv4DefaultSelector,
		Table:    TunRoutingTable,
		Action:   "add",
		Reason:   "send default IPv4 traffic through the TunWarden routing table before the kernel main table rule",
	}}
	if serverIP != "" {
		routes = append(routes, serverBypass)
		policyRules = append([]TunPolicyRulePlan{{
			Family:   "ipv4",
			Priority: ServerRulePriority,
			Selector: "to " + serverIP + "/32",
			Table:    MainRoutingTable,
			Action:   "add",
			Reason:   "keep VPN server traffic on the current uplink before the full-tunnel policy rule",
		}}, policyRules...)
	}

	dnsPlan := dnsPlan(s, device, normalizeDNSServers(opts.DNSServers))
	firewallPlan := firewallPlan(s, normalizeKillSwitchPolicy(opts.KillSwitchPolicy), device, serverIP)

	loopRisks := tunRouteLoopRisks(s)
	warnings := append([]string{}, s.Warnings...)
	warnings = append(warnings, tunSnapshotWarnings(s)...)
	warnings = append(warnings, tunDesiredStateWarnings(s, serverIP)...)
	warnings = append(warnings, dnsPlanWarnings(s, dnsPlan)...)
	warnings = append(warnings, firewallPlanWarnings(s, firewallPlan)...)
	warnings = append(warnings, loopRisks...)

	steps := []string{
		"Collect current host networking snapshot without requiring root",
		fmt.Sprintf("Plan TUN interface %s with MTU %d", device.Name, device.MTU),
		fmt.Sprintf("Plan routing table %s (%d) with IPv4 default route through %s", TunRoutingTable, TunRoutingTableID, device.Name),
	}
	if serverIP != "" {
		steps = append(steps, fmt.Sprintf("Plan policy rule priority %d for VPN server bypass via %s", ServerRulePriority, MainRoutingTable))
	}
	steps = append(steps,
		fmt.Sprintf("Plan policy rule priority %d for default IPv4 traffic via %s", TunRulePriority, TunRoutingTable),
		fmt.Sprintf("Plan DNS backend %s on link %s with server(s) %s", dnsPlan.Backend, dnsPlan.TargetLink, strings.Join(dnsPlan.Servers, ", ")),
		fmt.Sprintf("Plan nftables table %s %s with %d chain(s), %d rule(s), and %s kill-switch policy", firewallPlan.Family, firewallPlan.Table, len(firewallPlan.Chains), len(firewallPlan.Rules), firewallPlan.KillSwitch.Policy),
		"Leave TUN devices, routes, policy rules, DNS, nftables, firewall, and Xray process state unchanged in this dry-run",
	)

	rollbackSteps := []string{}
	rollbackSteps = append(rollbackSteps, firewallPlan.RollbackSteps...)
	rollbackSteps = append(rollbackSteps, dnsPlan.RollbackSteps...)
	rollbackSteps = append(rollbackSteps,
		fmt.Sprintf("Delete policy rule priority %d from all lookup %s if created by this transaction", TunRulePriority, TunRoutingTable),
	)
	if serverIP != "" {
		rollbackSteps = append(rollbackSteps,
			fmt.Sprintf("Delete policy rule priority %d to %s/32 lookup %s if created by this transaction", ServerRulePriority, serverIP, MainRoutingTable),
			fmt.Sprintf("Delete route %s/32 from table %s via %s dev %s if created by this transaction", serverIP, serverBypass.Table, serverBypass.Gateway, serverBypass.Interface),
		)
	}
	rollbackSteps = append(rollbackSteps,
		fmt.Sprintf("Delete route %s from table %s dev %s if created by this transaction", IPv4DefaultRoute, TunRoutingTable, device.Name),
		fmt.Sprintf("Delete TUN interface %s only if this transaction created it and ownership matches TunWarden", device.Name),
	)

	return TunPlan{
		Mode:          ModeTun,
		TunnelMode:    TunTunnelMode,
		ProfileID:     p.ID,
		ProfileName:   p.Name,
		Snapshot:      s,
		TunDevice:     device,
		Routes:        routes,
		PolicyRules:   policyRules,
		ServerBypass:  serverBypass,
		DNS:           dnsPlan,
		Firewall:      firewallPlan,
		LoopRisks:     loopRisks,
		Warnings:      warnings,
		Steps:         steps,
		RollbackSteps: rollbackSteps,
	}, nil
}

func serverBypassRoute(s snapshot.Snapshot, serverIP string) TunRoutePlan {
	if serverIP == "" {
		return TunRoutePlan{Family: "ipv4", Destination: "", Table: MainRoutingTable, Action: "skip", Reason: "VPN server bypass route is unavailable until server resolves to an IPv4 address"}
	}
	return TunRoutePlan{
		Family:      "ipv4",
		Destination: serverIP + "/32",
		Table:       MainRoutingTable,
		Interface:   s.DefaultIPv4Route.Interface,
		Gateway:     s.DefaultIPv4Route.Gateway,
		Action:      "add",
		Reason:      "pin VPN server traffic to the current default uplink outside the TUN path",
	}
}

func concreteServerBypassIP(s snapshot.Snapshot) string {
	if s.ServerRoute.Status != snapshot.StatusDetected {
		return ""
	}
	if net.ParseIP(s.ServerRoute.Destination) == nil {
		return ""
	}
	return s.ServerRoute.Destination
}

func tunRouteLoopRisks(s snapshot.Snapshot) []string {
	if s.ServerRoute.Status == snapshot.StatusDetected && s.DefaultIPv4Route.Interface != "" && s.ServerRoute.Interface == snapshot.DefaultTunName {
		return []string{"VPN server route already points at TunWarden TUN device; connecting would loop the control channel"}
	}
	return nil
}

func tunSnapshotWarnings(s snapshot.Snapshot) []string {
	warnings := []string{}
	if s.IPv4.Status != snapshot.StatusDetected {
		warnings = append(warnings, "IPv4 default route missing; TUN planning cannot claim full-tunnel IPv4 coverage")
	}
	if s.IPv6.Status == snapshot.StatusMissing {
		warnings = append(warnings, "IPv6 state is missing; initial TUN planning keeps IPv6 disabled or bypassed")
	}
	return warnings
}

func tunDesiredStateWarnings(s snapshot.Snapshot, serverIP string) []string {
	warnings := []string{}
	if serverIP == "" {
		warnings = append(warnings, "VPN server bypass route target is unresolved; connect must not apply full-tunnel routing until the server route is concrete")
	}
	return warnings
}

func dnsPlan(s snapshot.Snapshot, device TunDevicePlan, servers []string) TunDNSPlan {
	if s.DNS.Mode == snapshot.DNSModeSystemdResolved && s.DNS.SystemdResolved.Status == snapshot.StatusDetected {
		return TunDNSPlan{Backend: DNSBackendSystemdResolved, TargetLink: device.Name, Servers: servers, Action: DNSActionConfigure, Reason: "use systemd-resolved per-link DNS for full-tunnel DNS routing without writing /etc/resolv.conf", Rollback: DNSRollbackRestore, RollbackSteps: []string{
			fmt.Sprintf("Restore previous systemd-resolved per-link DNS state for %s where possible", device.Name),
			fmt.Sprintf("Flush TunWarden-owned DNS server/default-route/domain settings from %s if created by this transaction", device.Name),
		}}
	}
	return TunDNSPlan{Backend: DNSBackendSystemdResolved, TargetLink: device.Name, Servers: servers, Action: DNSActionBlocked, Reason: "systemd-resolved is required for first safe TUN DNS backend", Rollback: DNSRollbackRestore}
}

func dnsPlanWarnings(s snapshot.Snapshot, plan TunDNSPlan) []string {
	if plan.Action == DNSActionBlocked {
		return []string{"DNS desired state is blocked: " + plan.Reason}
	}
	return nil
}

func normalizeDNSServers(values []string) []string {
	servers := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if net.ParseIP(value) == nil {
			continue
		}
		if seen[value] {
			continue
		}
		seen[value] = true
		servers = append(servers, value)
	}
	if len(servers) == 0 {
		return []string{DefaultTunDNSServer}
	}
	return servers
}

func normalizeKillSwitchPolicy(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", KillSwitchPolicySoft:
		return KillSwitchPolicySoft
	case KillSwitchPolicyOff, KillSwitchPolicyStrict:
		return value
	default:
		return KillSwitchPolicySoft
	}
}

func firewallPlan(s snapshot.Snapshot, policy string, device TunDevicePlan, serverIP string) TunFirewallPlan {
	rules := []TunFirewallRulePlan{}
	if serverIP != "" {
		rules = append(rules, TunFirewallRulePlan{Chain: FirewallOutputChain, Expr: "ip daddr " + serverIP, Verdict: FirewallVerdictAccept, Action: FirewallActionAdd, Reason: "allow VPN server control traffic outside TUN before non-TUN blocking", Ownership: FirewallServerBypassOwner, RollbackKey: FirewallServerBypassKey})
	}
	rules = append(rules,
		TunFirewallRulePlan{Chain: FirewallOutputChain, Expr: "oifname \"" + device.Name + "\"", Verdict: FirewallVerdictAccept, Action: FirewallActionAdd, Reason: "allow traffic that egresses through the TunWarden TUN interface", Ownership: FirewallTunEgressOwner, RollbackKey: FirewallTunEgressKey},
		TunFirewallRulePlan{Chain: FirewallOutputChain, Expr: "oifname != \"" + device.Name + "\"", Verdict: FirewallVerdictReject, Action: FirewallActionAdd, Reason: "soft kill-switch rejects non-TUN traffic during transition and must restore direct connectivity on failure", Ownership: FirewallKillSwitchOwner, RollbackKey: FirewallKillSwitchKey},
	)

	action := FirewallTableAction
	reason := "create TunWarden-owned nftables firewall table"
	if s.NFTables.Availability.Status != snapshot.StatusDetected || s.NFTables.TunWardenTable.Status == snapshot.StatusUnknown {
		action = FirewallActionBlocked
		reason = "nftables availability is unknown; firewall mutation is unsafe until nft can be inspected"
	}
	return TunFirewallPlan{Backend: FirewallBackendNftables, Family: snapshot.DefaultNFTFamily, Table: snapshot.DefaultNFTTable, TableAction: action, Chains: []TunFirewallChainPlan{{Name: FirewallOutputChain, Type: FirewallChainTypeFilter, Hook: FirewallOutputHook, Priority: FirewallOutputPriority, Policy: FirewallDefaultChainPolicy, Action: action, Reason: "own outbound full-tunnel filtering inside the TunWarden nftables table"}}, Rules: rules, KillSwitch: TunKillSwitchPlan{Policy: policy, Action: "block non-TUN traffic according to selected kill-switch policy", Scope: "allow traffic through " + device.Name + " and the explicit VPN server bypass; block other non-TUN traffic according to policy", Recovery: "recover --execute --yes must be able to remove TunWarden-owned nftables state", Limitations: []string{"soft kill-switch is a transition guard only and must restore direct connectivity on failure"}}, Reason: reason, Rollback: FirewallRollbackRemove, RollbackSteps: []string{"Remove nftables table inet tunwarden if created by this transaction"}}
}

func firewallPlanWarnings(s snapshot.Snapshot, plan TunFirewallPlan) []string {
	if plan.TableAction == FirewallActionBlocked {
		return []string{"firewall desired state is blocked: " + plan.Reason}
	}
	return nil
}
