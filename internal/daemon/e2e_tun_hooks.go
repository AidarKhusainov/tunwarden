package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	netexecutor "github.com/AidarKhusainov/podlaz/internal/network/executor"
	"github.com/AidarKhusainov/podlaz/internal/network/planner"
)

const (
	e2eTunHookGateEnv  = "PODLAZ_E2E_TUN_HOOKS"
	e2eTunHookPhaseEnv = "PODLAZ_E2E_TUN_HOOK_PHASE"

	e2eTunHookRouteApplyPhase = "route-apply"
	e2eTunHookDNSApplyPhase   = "dns-apply"
)

func e2eTunHooksEnabled() bool {
	value := strings.TrimSpace(os.Getenv(e2eTunHookGateEnv))
	return value == "1" || strings.EqualFold(value, "true")
}

func e2eTunHookPhase() string {
	if !e2eTunHooksEnabled() {
		return ""
	}
	return strings.TrimSpace(os.Getenv(e2eTunHookPhaseEnv))
}

func validateE2ETunHookConfig() error {
	if !e2eTunHooksEnabled() {
		return nil
	}
	switch e2eTunHookPhase() {
	case e2eTunHookRouteApplyPhase, e2eTunHookDNSApplyPhase:
		return nil
	case "":
		return fmt.Errorf("%s is enabled but %s is empty", e2eTunHookGateEnv, e2eTunHookPhaseEnv)
	default:
		return fmt.Errorf("unsupported %s=%q", e2eTunHookPhaseEnv, e2eTunHookPhase())
	}
}

func maybeWrapE2ETunHookExecutor(executor netexecutor.DNSAwareTunExecutor) tunPlanExecutor {
	switch e2eTunHookPhase() {
	case e2eTunHookRouteApplyPhase:
		executor.Base.Routes = e2eHookRouteExecutor{delegate: executor.Base.Routes}
	case e2eTunHookDNSApplyPhase:
		executor.DNS = e2eHookDNSExecutor{delegate: executor.DNS}
	}
	return executor
}

type e2eHookRouteExecutor struct {
	delegate netexecutor.RouteExecutor
}

func (e e2eHookRouteExecutor) Add(ctx context.Context, plan planner.TunRoutePlan) (netexecutor.Step, error) {
	if e.delegate == nil {
		return netexecutor.Step{}, errors.New("missing route executor")
	}
	step, err := e.delegate.Add(ctx, plan)
	if err != nil {
		return step, err
	}
	return step, errors.New("E2E hook: route apply failed after a podlaz-owned route step was applied")
}

func (e e2eHookRouteExecutor) Verify(ctx context.Context, plan planner.TunRoutePlan) error {
	if e.delegate == nil {
		return errors.New("missing route executor")
	}
	return e.delegate.Verify(ctx, plan)
}

func (e e2eHookRouteExecutor) Rollback(ctx context.Context, plan planner.TunRoutePlan) error {
	if e.delegate == nil {
		return errors.New("missing route executor")
	}
	return e.delegate.Rollback(ctx, plan)
}

type e2eHookDNSExecutor struct {
	delegate netexecutor.DNSExecutor
}

func (e e2eHookDNSExecutor) Apply(ctx context.Context, plan planner.TunDNSPlan) (netexecutor.Step, error) {
	if e.delegate == nil {
		return netexecutor.Step{}, errors.New("missing DNS executor")
	}
	step, err := e.delegate.Apply(ctx, plan)
	if err != nil {
		return step, err
	}
	return step, errors.New("E2E hook: DNS apply failed after podlaz-owned per-link DNS was applied")
}

func (e e2eHookDNSExecutor) Verify(ctx context.Context, plan planner.TunDNSPlan) error {
	if e.delegate == nil {
		return errors.New("missing DNS executor")
	}
	return e.delegate.Verify(ctx, plan)
}

func (e e2eHookDNSExecutor) Rollback(ctx context.Context, plan planner.TunDNSPlan) error {
	if e.delegate == nil {
		return errors.New("missing DNS executor")
	}
	return e.delegate.Rollback(ctx, plan)
}
