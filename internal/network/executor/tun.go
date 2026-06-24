// Package executor applies already-planned privileged Linux networking changes.
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/AidarKhusainov/podlaz/internal/network/planner"
)

const (
	OwnerTunDevice  = "podlaz:tun-device"
	OwnerRoute      = "podlaz:route"
	OwnerPolicyRule = "podlaz:policy-rule"
)

const defaultCommandTimeout = 5 * time.Second

const (
	defaultTunDeviceUser  = "podlaz-xray"
	defaultTunDeviceGroup = "podlaz-xray"
)

// CommandResult is a low-level command execution result.
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// CommandRunner runs privileged host commands for the OS-backed executor.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
}

// OSRunner executes commands through os/exec.
type OSRunner struct{}

// Run executes a host command and captures stdout, stderr, and exit code.
func (OSRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := CommandResult{
		Stdout: strings.TrimSpace(stdout.String()),
		Stderr: strings.TrimSpace(stderr.String()),
	}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
	} else {
		result.ExitCode = -1
	}
	return result, err
}

// TunDeviceExecutor owns TUN interface creation, verification, and cleanup.
type TunDeviceExecutor interface {
	Create(ctx context.Context, plan planner.TunDevicePlan) (Step, error)
	Verify(ctx context.Context, plan planner.TunDevicePlan) error
	Rollback(ctx context.Context, plan planner.TunDevicePlan) error
}

// RouteExecutor owns route apply, verification, and cleanup.
type RouteExecutor interface {
	Add(ctx context.Context, plan planner.TunRoutePlan) (Step, error)
	Verify(ctx context.Context, plan planner.TunRoutePlan) error
	Rollback(ctx context.Context, plan planner.TunRoutePlan) error
}

// PolicyRuleExecutor owns policy-rule apply, verification, and cleanup.
type PolicyRuleExecutor interface {
	Add(ctx context.Context, plan planner.TunPolicyRulePlan) (Step, error)
	Verify(ctx context.Context, plan planner.TunPolicyRulePlan) error
	Rollback(ctx context.Context, plan planner.TunPolicyRulePlan) error
}

// Step records one applied podlaz-owned networking mutation.
type Step struct {
	Kind        string
	Target      string
	Description string
	Owner       string
}

// TunExecutor applies the TUN, route, and policy-rule parts of a TUN plan.
type TunExecutor struct {
	TunDevice   TunDeviceExecutor
	Routes      RouteExecutor
	PolicyRules PolicyRuleExecutor
}

// NewOSExecutor returns the Linux iproute2-backed executor.
func NewOSExecutor() TunExecutor {
	runner := OSRunner{}
	return TunExecutor{
		TunDevice:   IPTunDeviceExecutor{Runner: runner, DeviceUser: defaultTunDeviceUser, DeviceGroup: defaultTunDeviceGroup},
		Routes:      IPRouteExecutor{Runner: runner},
		PolicyRules: IPPolicyRuleExecutor{Runner: runner},
	}
}

// Apply applies TUN, routes, and policy rules from the already-inspected plan.
func (e TunExecutor) Apply(ctx context.Context, plan planner.TunPlan) ([]Step, error) {
	if err := e.validate(); err != nil {
		return nil, err
	}
	steps := make([]Step, 0, 1+len(plan.Routes)+len(plan.PolicyRules))

	step, err := e.TunDevice.Create(ctx, plan.TunDevice)
	if err != nil {
		return steps, err
	}
	steps = append(steps, step)

	for _, route := range plan.Routes {
		if route.Action != "add" {
			continue
		}
		step, err := e.Routes.Add(ctx, route)
		if err != nil {
			return steps, err
		}
		steps = append(steps, step)
	}
	for _, rule := range plan.PolicyRules {
		if rule.Action != "add" {
			continue
		}
		step, err := e.PolicyRules.Add(ctx, rule)
		if err != nil {
			return steps, err
		}
		steps = append(steps, step)
	}
	return steps, nil
}

// Verify checks that every applied TUN, route, and policy-rule target is visible.
func (e TunExecutor) Verify(ctx context.Context, plan planner.TunPlan) error {
	if err := e.validate(); err != nil {
		return err
	}
	if err := e.TunDevice.Verify(ctx, plan.TunDevice); err != nil {
		return err
	}
	for _, route := range plan.Routes {
		if route.Action != "add" {
			continue
		}
		if err := e.Routes.Verify(ctx, route); err != nil {
			return err
		}
	}
	for _, rule := range plan.PolicyRules {
		if rule.Action != "add" {
			continue
		}
		if err := e.PolicyRules.Verify(ctx, rule); err != nil {
			return err
		}
	}
	return nil
}

// Rollback removes policy rules, routes, and TUN interface in reverse dependency order.
func (e TunExecutor) Rollback(ctx context.Context, plan planner.TunPlan) error {
	if err := e.validate(); err != nil {
		return err
	}
	var errs []error
	for i := len(plan.PolicyRules) - 1; i >= 0; i-- {
		rule := plan.PolicyRules[i]
		if rule.Action != "add" {
			continue
		}
		if err := e.PolicyRules.Rollback(ctx, rule); err != nil {
			errs = append(errs, err)
		}
	}
	for i := len(plan.Routes) - 1; i >= 0; i-- {
		route := plan.Routes[i]
		if route.Action != "add" {
			continue
		}
		if err := e.Routes.Rollback(ctx, route); err != nil {
			errs = append(errs, err)
		}
	}
	if err := e.TunDevice.Rollback(ctx, plan.TunDevice); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func (e TunExecutor) validate() error {
	if e.TunDevice == nil {
		return errors.New("missing TUN device executor")
	}
	if e.Routes == nil {
		return errors.New("missing route executor")
	}
	if e.PolicyRules == nil {
		return errors.New("missing policy-rule executor")
	}
	return nil
}

// IPTunDeviceExecutor applies TUN device operations through iproute2.
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
