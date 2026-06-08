package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/AidarKhusainov/tunwarden/internal/network/planner"
	netsnapshot "github.com/AidarKhusainov/tunwarden/internal/network/snapshot"
	"github.com/AidarKhusainov/tunwarden/internal/profile"
	"github.com/AidarKhusainov/tunwarden/internal/render"
)

func runPlanCommand(ctx context.Context, args []string, stdout io.Writer, opts options) error {
	if isHelp(args) {
		printPlanHelp(stdout)
		return nil
	}

	parsed, err := parsePlanArgs(args)
	if err != nil {
		return err
	}

	store, err := profile.NewStore(opts.profileStorePath)
	if err != nil {
		return err
	}
	p, err := store.Get(parsed.profileID)
	if err != nil {
		return profileCommandError(err)
	}

	switch parsed.mode {
	case planner.ModeProxyOnly:
		proxyPlan, err := planner.PlanProxyOnly(p)
		if err != nil {
			return usageError("%s", err.Error())
		}
		if parsed.jsonOutput {
			return writeJSON(stdout, proxyOnlyPlanJSON(proxyPlan))
		}
		renderProxyOnlyPlan(stdout, proxyPlan)
	case planner.ModeTun:
		collect := opts.systemSnapshot
		if collect == nil {
			collect = netsnapshot.Collect
		}
		hostSnapshot := collect(ctx, netsnapshot.Options{Server: p.Server, TunNames: []string{netsnapshot.DefaultTunName}})
		tunPlan, err := planner.PlanTun(p, hostSnapshot)
		if err != nil {
			return usageError("%s", err.Error())
		}
		if parsed.jsonOutput {
			return writeJSON(stdout, tunPlanJSON(tunPlan))
		}
		renderTunPlan(stdout, tunPlan)
	}
	return nil
}

type planArgs struct {
	mode       string
	profileID  string
	jsonOutput bool
}

func parsePlanArgs(args []string) (planArgs, error) {
	var parsed planArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value, hasInlineValue := cutFlagValue(arg)
		switch {
		case arg == "--mode" || strings.HasPrefix(arg, "--mode="):
			v, next, err := flagValue("plan --mode", args, i, value, hasInlineValue)
			if err != nil {
				return parsed, err
			}
			parsed.mode = strings.ToLower(strings.TrimSpace(v))
			i = next
		case arg == "--json":
			parsed.jsonOutput = true
		default:
			if strings.HasPrefix(arg, "-") {
				return parsed, usageError("unsupported plan argument %q", arg)
			}
			if parsed.profileID != "" {
				return parsed, usageError("plan accepts exactly one profile id")
			}
			parsed.profileID = arg
		}
	}

	if parsed.mode == "" {
		return parsed, usageError("plan requires --mode proxy-only or tun")
	}
	if parsed.mode != planner.ModeProxyOnly && parsed.mode != planner.ModeTun {
		return parsed, usageError("unsupported plan mode %q", parsed.mode)
	}
	if parsed.profileID == "" {
		return parsed, usageError("plan requires a profile id")
	}
	return parsed, nil
}

func renderProxyOnlyPlan(stdout io.Writer, proxyPlan planner.ProxyOnlyPlan) {
	fmt.Fprintln(stdout, "Proxy-only plan")
	fmt.Fprintf(stdout, "Profile: %s\n", render.Redact(proxyPlan.ProfileName))
	fmt.Fprintf(stdout, "Profile ID: %s\n", render.Redact(proxyPlan.ProfileID))
	fmt.Fprintf(stdout, "Mode: %s\n", proxyPlan.Mode)
	fmt.Fprintf(stdout, "Will generate runtime Xray config: %s\n", proxyPlan.RuntimeConfigPath)
	for _, listener := range proxyPlan.Listeners {
		fmt.Fprintf(stdout, "Will listen on %s: %s\n", listener.Protocol, listener.Endpoint())
	}
	fmt.Fprintln(stdout, planner.NoSystemNetworkingPlan)
	fmt.Fprintln(stdout, "Will not start Xray or write the generated config in this dry-run.")
	if len(proxyPlan.Warnings) > 0 {
		fmt.Fprintf(stdout, "Warnings: %d\n", len(proxyPlan.Warnings))
		for _, warning := range proxyPlan.Warnings {
			fmt.Fprintf(stdout, "- %s\n", render.Redact(warning))
		}
	}
}

func renderTunPlan(stdout io.Writer, tunPlan planner.TunPlan) {
	s := tunPlan.Snapshot
	fmt.Fprintln(stdout, "TUN planning snapshot")
	fmt.Fprintf(stdout, "Profile: %s\n", render.Redact(tunPlan.ProfileName))
	fmt.Fprintf(stdout, "Profile ID: %s\n", render.Redact(tunPlan.ProfileID))
	fmt.Fprintf(stdout, "Mode: %s\n", tunPlan.Mode)
	fmt.Fprintln(stdout, "Read-only: will not create TUN devices, change routes, change DNS, change nftables, start Xray, or write runtime config.")
	fmt.Fprintf(stdout, "Default IPv4 route: %s\n", renderRoute(s.DefaultIPv4))
	fmt.Fprintf(stdout, "Default interface: %s\n", renderDefaultInterface(s.DefaultIPv4))
	fmt.Fprintf(stdout, "Default IPv6 route: %s\n", renderRoute(s.DefaultIPv6))
	fmt.Fprintf(stdout, "Route to VPN server candidate: %s\n", renderRoute(s.ServerRoute))
	fmt.Fprintf(stdout, "DNS mode: %s (%s)\n", render.Redact(s.DNS.Mode), renderFinding(s.DNS.Resolved))
	fmt.Fprintf(stdout, "NetworkManager: %s\n", renderNetworkManager(s.NetworkManager))
	fmt.Fprintf(stdout, "nftables: %s\n", renderFinding(s.Nftables.Availability))
	fmt.Fprintf(stdout, "TunWarden nftables table: %s\n", renderFinding(s.Nftables.TunWardenTable))
	fmt.Fprintf(stdout, "IPv4 assumption: %s\n", renderFinding(s.IPv4))
	fmt.Fprintf(stdout, "IPv6 assumption: %s\n", renderFinding(s.IPv6))
	fmt.Fprintln(stdout, "TunWarden TUN devices:")
	for _, device := range s.TunDevices {
		fmt.Fprintf(stdout, "- %s: %s\n", render.Redact(device.Name), renderStatusDetail(device.Status, device.Detail, device.Raw))
	}
	if len(s.StaleResources) == 0 {
		fmt.Fprintln(stdout, "Stale TunWarden-owned resources: none detected")
	} else {
		fmt.Fprintf(stdout, "Stale TunWarden-owned resources: %d detected\n", len(s.StaleResources))
		for _, stale := range s.StaleResources {
			fmt.Fprintf(stdout, "- %s %s: %s\n", render.Redact(stale.Kind), render.Redact(stale.Name), renderStatusDetail(stale.Status, stale.Detail, ""))
		}
	}
	if len(tunPlan.Warnings) > 0 {
		fmt.Fprintf(stdout, "Warnings: %d\n", len(tunPlan.Warnings))
		for _, warning := range tunPlan.Warnings {
			fmt.Fprintf(stdout, "- %s\n", render.Redact(warning))
		}
	}
}

func proxyOnlyPlanJSON(proxyPlan planner.ProxyOnlyPlan) map[string]any {
	warnings := redactedStrings(proxyPlan.Warnings)
	status := "ok"
	if len(warnings) > 0 {
		status = "warn"
	}
	return map[string]any{
		"schema_version": "v1",
		"status":         status,
		"warnings":       warnings,
		"errors":         []string{},
		"mode":           proxyPlan.Mode,
		"plan": map[string]any{
			"profile": map[string]any{
				"id":   render.Redact(proxyPlan.ProfileID),
				"name": render.Redact(proxyPlan.ProfileName),
			},
			"runtime_config_path":        proxyPlan.RuntimeConfigPath,
			"listeners":                  listenersForJSON(proxyPlan.Listeners),
			"writes_config":              false,
			"starts_xray":                false,
			"modifies_system_networking": false,
			"system_networking":          "will not modify TUN, routes, DNS, nftables, or firewall",
		},
		"steps":          redactedStrings(proxyPlan.Steps),
		"rollback_steps": redactedStrings(proxyPlan.RollbackSteps),
	}
}

func tunPlanJSON(tunPlan planner.TunPlan) map[string]any {
	warnings := redactedStrings(tunPlan.Warnings)
	status := "ok"
	if len(warnings) > 0 {
		status = "warn"
	}
	return map[string]any{
		"schema_version": "v1",
		"status":         status,
		"warnings":       warnings,
		"errors":         []string{},
		"mode":           tunPlan.Mode,
		"plan": map[string]any{
			"profile": map[string]any{
				"id":   render.Redact(tunPlan.ProfileID),
				"name": render.Redact(tunPlan.ProfileName),
			},
			"writes_config":              false,
			"starts_xray":                false,
			"modifies_system_networking": false,
			"snapshot":                   snapshotForJSON(tunPlan.Snapshot),
		},
		"steps":          redactedStrings(tunPlan.Steps),
		"rollback_steps": redactedStrings(tunPlan.RollbackSteps),
	}
}

func listenersForJSON(listeners []planner.Listener) []map[string]any {
	out := make([]map[string]any, len(listeners))
	for i, listener := range listeners {
		out[i] = map[string]any{
			"protocol": strings.ToLower(listener.Protocol),
			"address":  listener.Address,
			"port":     listener.Port,
		}
	}
	return out
}

func snapshotForJSON(s netsnapshot.Snapshot) map[string]any {
	return map[string]any{
		"os":                 render.Redact(s.OS),
		"default_ipv4_route": routeForJSON(s.DefaultIPv4),
		"default_ipv6_route": routeForJSON(s.DefaultIPv6),
		"server_route":       routeForJSON(s.ServerRoute),
		"dns": map[string]any{
			"mode":             render.Redact(s.DNS.Mode),
			"systemd_resolved": findingForJSON(s.DNS.Resolved),
		},
		"network_manager": map[string]any{
			"finding": findingForJSON(s.NetworkManager.Finding),
			"state":   render.Redact(s.NetworkManager.State),
		},
		"nftables": map[string]any{
			"availability":    findingForJSON(s.Nftables.Availability),
			"tunwarden_table": findingForJSON(s.Nftables.TunWardenTable),
		},
		"tun_devices":     tunDevicesForJSON(s.TunDevices),
		"ipv4":            findingForJSON(s.IPv4),
		"ipv6":            findingForJSON(s.IPv6),
		"stale_resources": staleResourcesForJSON(s.StaleResources),
	}
}

func routeForJSON(route netsnapshot.Route) map[string]any {
	return map[string]any{
		"status":      string(route.Status),
		"family":      render.Redact(route.Family),
		"destination": render.Redact(route.Destination),
		"interface":   render.Redact(route.Interface),
		"gateway":     render.Redact(route.Gateway),
		"raw":         render.Redact(route.Raw),
		"detail":      render.Redact(route.Detail),
	}
}

func findingForJSON(f netsnapshot.Finding) map[string]any {
	return map[string]any{
		"status":  string(f.Status),
		"summary": render.Redact(f.Summary),
		"detail":  render.Redact(f.Detail),
	}
}

func tunDevicesForJSON(devices []netsnapshot.TunDevice) []map[string]any {
	out := make([]map[string]any, len(devices))
	for i, device := range devices {
		out[i] = map[string]any{
			"name":   render.Redact(device.Name),
			"status": string(device.Status),
			"detail": render.Redact(device.Detail),
			"raw":    render.Redact(device.Raw),
		}
	}
	return out
}

func staleResourcesForJSON(resources []netsnapshot.StaleResource) []map[string]any {
	out := make([]map[string]any, len(resources))
	for i, resource := range resources {
		out[i] = map[string]any{
			"kind":   render.Redact(resource.Kind),
			"name":   render.Redact(resource.Name),
			"status": string(resource.Status),
			"detail": render.Redact(resource.Detail),
		}
	}
	return out
}

func renderRoute(route netsnapshot.Route) string {
	if route.Status == netsnapshot.StatusDetected {
		parts := []string{string(route.Status)}
		if route.Interface != "" {
			parts = append(parts, "dev "+route.Interface)
		}
		if route.Gateway != "" {
			parts = append(parts, "via "+route.Gateway)
		}
		if route.Raw != "" {
			parts = append(parts, "raw: "+route.Raw)
		}
		return render.Redact(strings.Join(parts, ", "))
	}
	return renderStatusDetail(route.Status, route.Detail, route.Raw)
}

func renderDefaultInterface(route netsnapshot.Route) string {
	if route.Status == netsnapshot.StatusDetected && route.Interface != "" {
		return render.Redact(route.Interface)
	}
	return renderStatusDetail(route.Status, route.Detail, route.Raw)
}

func renderNetworkManager(nm netsnapshot.NetworkManager) string {
	line := renderFinding(nm.Finding)
	if nm.State != "" {
		line += " state=" + nm.State
	}
	return render.Redact(line)
}

func renderFinding(f netsnapshot.Finding) string {
	return renderStatusDetail(f.Status, f.Summary, f.Detail)
}

func renderStatusDetail(status netsnapshot.Status, primary, secondary string) string {
	parts := []string{string(status)}
	if primary != "" {
		parts = append(parts, primary)
	}
	if secondary != "" {
		parts = append(parts, secondary)
	}
	return render.Redact(strings.Join(parts, ": "))
}

func redactedStrings(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = render.Redact(value)
	}
	return out
}

func printPlanHelp(w io.Writer) {
	fmt.Fprint(w, `Usage:
  tunwarden plan --mode proxy-only <profile-id> [--json]
  tunwarden plan --mode tun <profile-id> [--json]

Print a read-only connection plan for a stored profile. Proxy-only mode builds an
inspectable generated Xray config in memory. TUN mode collects a read-only system
snapshot for full-tunnel planning and explains the current default route, default
interface, DNS backend, NetworkManager state, nftables state, IPv4/IPv6
assumptions, and stale TunWarden-owned resources.

Implemented:
  proxy-only plans for supported stored VLESS profiles, human output, JSON
  output, deterministic generated Xray config fixtures, and explicit no TUN,
  route, DNS, nftables, or firewall mutation.

  TUN planning snapshots with no TUN creation, route mutation, DNS mutation,
  nftables mutation, Xray process start, or runtime config write.

Not implemented yet:
  writing generated config files, Xray binary discovery/version checks, Xray
  process start, full-tunnel execution
`)
}
