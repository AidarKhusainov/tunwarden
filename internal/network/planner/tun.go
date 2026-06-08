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
	TunRulePriority     = 51820
	ServerRulePriority  = 51819
	MainRoutingTable    = "main"
	IPv4DefaultRoute    = "default"
	IPv4DefaultSelector = "from all"
)

// TunDevicePlan describes the TUN interface desired state for a dry-run plan.
type TunDevicePlan struct {
	Name   string
	MTU    int
	Action string
	Reason string
}

// TunRoutePlan describes an intended route without applying it.
type TunRoutePlan struct {
	Family      string
	Destination string
	Table       string
	Interface   string
	Gateway     string
	Action      string
	Reason      string
}

// TunPolicyRulePlan describes an intended policy routing rule without applying it.
type TunPolicyRulePlan struct {
	Family   string
	Priority int
	Selector string
	Table    string
	Action   string
	Reason   string
}

// TunPlan is the inspectable read-only full-tunnel route/TUN dry-run plan.
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
	LoopRisks     []string
	Warnings      []string
	Steps         []string
	RollbackSteps []string
}

// PlanTun builds a read-only full-tunnel route/TUN plan from an already collected system snapshot.
func PlanTun(p profile.Profile, s snapshot.Snapshot) (TunPlan, error) {
	if err := profile.Validate(p); err != nil {
		return TunPlan{}, err
	}

	device := TunDevicePlan{
		Name:   snapshot.DefaultTunName,
		MTU:    DefaultTunMTU,
		Action: "create",
		Reason: "stable TunWarden full-tunnel interface",
	}
	serverTarget := serverBypassTarget(p, s)
	serverDestination := serverBypassDestination(serverTarget)
	serverSelector := serverBypassSelector(serverTarget)

	policyRules := []TunPolicyRulePlan{
		{
			Family:   "ipv4",
			Priority: ServerRulePriority,
			Selector: serverSelector,
			Table:    MainRoutingTable,
			Action:   "add",
			Reason:   "keep VPN server traffic on the current uplink before full-tunnel policy routing",
		},
		{
			Family:   "ipv4",
			Priority: TunRulePriority,
			Selector: IPv4DefaultSelector,
			Table:    TunRoutingTable,
			Action:   "add",
			Reason:   "send default IPv4 traffic through the TunWarden routing table",
		},
	}

	routes := []TunRoutePlan{
		{
			Family:      "ipv4",
			Destination: IPv4DefaultRoute,
			Table:       TunRoutingTable,
			Interface:   snapshot.DefaultTunName,
			Action:      "add",
			Reason:      "route default IPv4 traffic through the TunWarden TUN interface",
		},
	}

	serverBypass := TunRoutePlan{
		Family:      "ipv4",
		Destination: serverDestination,
		Table:       MainRoutingTable,
		Interface:   s.DefaultIPv4.Interface,
		Gateway:     s.DefaultIPv4.Gateway,
		Action:      "add",
		Reason:      "pin VPN server traffic to the current default uplink outside the TUN path",
	}
	if serverTarget != "" {
		routes = append(routes, serverBypass)
	}

	loopRisks := tunRouteLoopRisks(s)
	warnings := append([]string{}, s.Warnings...)
	warnings = append(warnings, tunSnapshotWarnings(s)...)
	warnings = append(warnings, tunDesiredStateWarnings(s, serverTarget)...)
	warnings = append(warnings, loopRisks...)

	return TunPlan{
		Mode:         ModeTun,
		TunnelMode:   TunTunnelMode,
		ProfileID:    p.ID,
		ProfileName:  p.Name,
		Snapshot:     s,
		TunDevice:    device,
		Routes:       routes,
		PolicyRules:  policyRules,
		ServerBypass: serverBypass,
		LoopRisks:    loopRisks,
		Warnings:     compactWarnings(warnings),
		Steps: []string{
			"Collect current host networking snapshot without requiring root",
			fmt.Sprintf("Plan TUN interface %s with MTU %d", device.Name, device.MTU),
			fmt.Sprintf("Plan routing table %s (%d) with IPv4 default route through %s", TunRoutingTable, TunRoutingTableID, device.Name),
			fmt.Sprintf("Plan policy rule priority %d for VPN server bypass via %s", ServerRulePriority, MainRoutingTable),
			fmt.Sprintf("Plan policy rule priority %d for default IPv4 traffic via %s", TunRulePriority, TunRoutingTable),
			"Leave TUN devices, routes, policy rules, DNS, nftables, firewall, and Xray process state unchanged in this dry-run",
		},
		RollbackSteps: rollbackSteps(device, routes, policyRules),
	}, nil
}

func tunSnapshotWarnings(s snapshot.Snapshot) []string {
	var warnings []string
	if s.DefaultIPv4.Status != snapshot.StatusDetected {
		warnings = append(warnings, fmt.Sprintf("IPv4 default route is %s; full-tunnel planning cannot select a stable uplink yet", s.DefaultIPv4.Status))
	}
	if s.ServerRoute.Status != snapshot.StatusDetected {
		warnings = append(warnings, fmt.Sprintf("route to VPN server candidate is %s; server bypass route planning is incomplete", s.ServerRoute.Status))
	}
	if s.DNS.Resolved.Status != snapshot.StatusDetected {
		warnings = append(warnings, fmt.Sprintf("systemd-resolved state is %s; DNS planning will need fallback handling", s.DNS.Resolved.Status))
	}
	if s.Nftables.Availability.Status != snapshot.StatusDetected {
		warnings = append(warnings, fmt.Sprintf("nftables availability is %s; firewall and kill-switch planning is incomplete", s.Nftables.Availability.Status))
	}
	if len(s.StaleResources) > 0 {
		warnings = append(warnings, fmt.Sprintf("found %d stale TunWarden-owned resource(s); recover should inspect them before applying TUN mode", len(s.StaleResources)))
	}
	if s.IPv6.Status != snapshot.StatusDetected {
		warnings = append(warnings, fmt.Sprintf("IPv6 state is %s; initial TUN planning keeps IPv6 disabled or bypassed", s.IPv6.Status))
	}
	return compactWarnings(warnings)
}

func tunDesiredStateWarnings(s snapshot.Snapshot, serverTarget string) []string {
	var warnings []string
	for _, device := range s.TunDevices {
		if device.Name == snapshot.DefaultTunName && device.Status == snapshot.StatusDetected {
			warnings = append(warnings, fmt.Sprintf("TunWarden TUN device %s already exists; recover or validate ownership before applying the planned create step", device.Name))
		}
	}
	if s.DefaultIPv4.Status == snapshot.StatusDetected && s.DefaultIPv4.Interface == snapshot.DefaultTunName {
		warnings = append(warnings, "current default IPv4 route already points at tunwarden0; applying another full-tunnel plan could preserve a route loop")
	}
	if s.DefaultIPv4.Status == snapshot.StatusDetected && s.DefaultIPv4.Interface == "" {
		warnings = append(warnings, "default IPv4 route did not expose an interface; VPN server bypass cannot be applied safely yet")
	}
	if s.DefaultIPv4.Status == snapshot.StatusDetected && s.DefaultIPv4.Gateway == "" {
		warnings = append(warnings, "default IPv4 route did not expose a gateway; VPN server bypass can only pin the uplink interface")
	}
	if serverTarget == "" {
		warnings = append(warnings, "VPN server bypass target is unknown; resolve the server route before applying full-tunnel routing")
	} else if net.ParseIP(strings.TrimSuffix(serverTarget, "/32")) == nil {
		warnings = append(warnings, fmt.Sprintf("VPN server bypass target %q is not a concrete IP address; hostname resolution must succeed before apply", serverTarget))
	}
	return compactWarnings(warnings)
}

func tunRouteLoopRisks(s snapshot.Snapshot) []string {
	var risks []string
	if s.ServerRoute.Status == snapshot.StatusDetected && s.ServerRoute.Interface == snapshot.DefaultTunName {
		risks = append(risks, "route to VPN server candidate points at tunwarden0; this would create a routing loop")
	}
	if s.DefaultIPv4.Status == snapshot.StatusDetected && s.DefaultIPv4.Interface == snapshot.DefaultTunName {
		risks = append(risks, "current default IPv4 route points at tunwarden0; full-tunnel planning needs a direct uplink snapshot")
	}
	return compactWarnings(risks)
}

func serverBypassTarget(p profile.Profile, s snapshot.Snapshot) string {
	if target := firstIPFromRoute(s.ServerRoute); target != "" {
		return target
	}
	if s.ServerRoute.Destination != "" && s.ServerRoute.Destination != "server" {
		return s.ServerRoute.Destination
	}
	return strings.TrimSpace(p.Server)
}

func firstIPFromRoute(route snapshot.Route) string {
	for _, value := range append(strings.Fields(route.Raw), strings.Fields(route.Detail)...) {
		value = strings.Trim(value, " ,;()[]")
		if ip := net.ParseIP(value); ip != nil {
			return ip.String()
		}
	}
	if ip := net.ParseIP(strings.TrimSpace(route.Destination)); ip != nil {
		return ip.String()
	}
	return ""
}

func serverBypassDestination(target string) string {
	if target == "" {
		return "unknown"
	}
	if ip := net.ParseIP(strings.TrimSpace(target)); ip != nil {
		return ip.String() + "/32"
	}
	return target
}

func serverBypassSelector(target string) string {
	destination := serverBypassDestination(target)
	if destination == "unknown" {
		return "to <server-ip>"
	}
	return "to " + destination
}

func rollbackSteps(device TunDevicePlan, routes []TunRoutePlan, rules []TunPolicyRulePlan) []string {
	steps := make([]string, 0, len(routes)+len(rules)+1)
	for i := len(rules) - 1; i >= 0; i-- {
		rule := rules[i]
		steps = append(steps, fmt.Sprintf("Delete policy rule priority %d %s lookup %s if created by this transaction", rule.Priority, rule.Selector, rule.Table))
	}
	for i := len(routes) - 1; i >= 0; i-- {
		route := routes[i]
		steps = append(steps, rollbackRouteStep(route))
	}
	steps = append(steps, fmt.Sprintf("Delete TUN interface %s only if this transaction created it and ownership matches TunWarden", device.Name))
	return steps
}

func rollbackRouteStep(route TunRoutePlan) string {
	parts := []string{"Delete route", route.Destination}
	if route.Table != "" {
		parts = append(parts, "from table", route.Table)
	}
	if route.Gateway != "" {
		parts = append(parts, "via", route.Gateway)
	}
	if route.Interface != "" {
		parts = append(parts, "dev", route.Interface)
	}
	parts = append(parts, "if created by this transaction")
	return strings.Join(parts, " ")
}

func compactWarnings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
