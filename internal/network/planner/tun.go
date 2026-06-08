package planner

import (
	"fmt"
	"strings"

	"github.com/AidarKhusainov/tunwarden/internal/network/snapshot"
	"github.com/AidarKhusainov/tunwarden/internal/profile"
)

const (
	ModeTun = "tun"
)

// TunPlan is the inspectable read-only preview for full-tunnel planning inputs.
type TunPlan struct {
	Mode          string
	ProfileID     string
	ProfileName   string
	Snapshot      snapshot.Snapshot
	Warnings      []string
	Steps         []string
	RollbackSteps []string
}

// PlanTun builds a read-only TUN planning preview from an already collected system snapshot.
func PlanTun(p profile.Profile, s snapshot.Snapshot) (TunPlan, error) {
	if err := profile.Validate(p); err != nil {
		return TunPlan{}, err
	}

	warnings := append([]string{}, s.Warnings...)
	warnings = append(warnings, tunSnapshotWarnings(s)...)

	return TunPlan{
		Mode:        ModeTun,
		ProfileID:   p.ID,
		ProfileName: p.Name,
		Snapshot:    s,
		Warnings:    warnings,
		Steps: []string{
			"Read current host networking snapshot without requiring root",
			"Inspect default route, default interface, and route to VPN server candidate",
			"Inspect DNS backend, NetworkManager advisory state, nftables availability, and TunWarden-owned resources",
			"Leave TUN devices, routes, DNS, nftables, firewall, and Xray process state unchanged",
		},
		RollbackSteps: []string{},
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
	if s.ServerRoute.Status == snapshot.StatusDetected && s.ServerRoute.Interface == snapshot.DefaultTunName {
		warnings = append(warnings, "route to VPN server candidate points at tunwarden0; this would create a routing loop")
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
