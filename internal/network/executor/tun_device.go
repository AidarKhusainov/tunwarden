package executor

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/AidarKhusainov/podlaz/internal/network/planner"
)

const (
	defaultTunDeviceUser  = "podlaz-xray"
	defaultTunDeviceGroup = "podlaz-xray"
)

type IPTunDeviceExecutor struct {
	Runner      CommandRunner
	DeviceUser  string
	DeviceGroup string
}

func (e IPTunDeviceExecutor) Create(ctx context.Context, plan planner.TunDevicePlan) (Step, error) {
	if plan.Name == "" {
		return Step{}, errors.New("missing TUN device name")
	}
	args := []string{"tuntap", "add", "dev", plan.Name, "mode", "tun"}
	if user := strings.TrimSpace(e.DeviceUser); user != "" {
		args = append(args, "user", user)
	}
	if group := strings.TrimSpace(e.DeviceGroup); group != "" {
		args = append(args, "group", group)
	}
	if err := e.run(ctx, "ip", args...); err != nil {
		return Step{}, fmt.Errorf("create TUN device %s: %w", plan.Name, err)
	}
	if plan.MTU > 0 {
		if err := e.run(ctx, "ip", "link", "set", "dev", plan.Name, "mtu", strconv.Itoa(plan.MTU)); err != nil {
			return Step{}, fmt.Errorf("set TUN device %s MTU: %w", plan.Name, err)
		}
	}
	if err := e.run(ctx, "ip", "link", "set", "dev", plan.Name, "up"); err != nil {
		return Step{}, fmt.Errorf("bring TUN device %s up: %w", plan.Name, err)
	}
	return Step{Kind: "tun-device", Target: plan.Name, Description: plan.Reason, Owner: OwnerTunDevice}, nil
}

func (e IPTunDeviceExecutor) Verify(ctx context.Context, plan planner.TunDevicePlan) error {
	if err := e.run(ctx, "ip", "link", "show", "dev", plan.Name); err != nil {
		return fmt.Errorf("verify TUN device %s: %w", plan.Name, err)
	}
	return nil
}

func (e IPTunDeviceExecutor) Rollback(ctx context.Context, plan planner.TunDevicePlan) error {
	if plan.Name == "" {
		return nil
	}
	if err := e.run(ctx, "ip", "link", "del", "dev", plan.Name); err != nil && !resourceMissing(err) {
		return fmt.Errorf("delete TUN device %s: %w", plan.Name, err)
	}
	return nil
}

func (e IPTunDeviceExecutor) run(ctx context.Context, name string, args ...string) error {
	return runCommand(ctx, e.Runner, name, args...)
}
