package daemon

import (
	"context"
	"strings"
	"testing"

	netexecutor "github.com/AidarKhusainov/podlaz/internal/network/executor"
	"github.com/AidarKhusainov/podlaz/internal/network/planner"
)

func TestValidateE2ETunHookConfigRejectsUnknownPhase(t *testing.T) {
	t.Setenv(e2eTunHookGateEnv, "true")
	t.Setenv(e2eTunHookPhaseEnv, "unknown")

	err := validateE2ETunHookConfig()
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported hook phase error, got %v", err)
	}
}

func TestE2ERouteHookFailsAfterDelegateApply(t *testing.T) {
	executor := e2eHookRouteExecutor{delegate: recordingRouteExecutor{}}
	step, err := executor.Add(context.Background(), planner.TunRoutePlan{Destination: "default", Table: planner.TunRoutingTable})
	if err == nil || !strings.Contains(err.Error(), "route apply") {
		t.Fatalf("expected route hook error, got %v", err)
	}
	if step.Kind != "route" || step.Owner != netexecutor.OwnerRoute {
		t.Fatalf("expected route step to be preserved, got %#v", step)
	}
}

func TestE2EDNSHookFailsAfterDelegateApply(t *testing.T) {
	executor := e2eHookDNSExecutor{delegate: recordingDNSExecutor{}}
	step, err := executor.Apply(context.Background(), planner.TunDNSPlan{TargetLink: "podlaz0", Action: planner.DNSActionConfigure, Servers: []string{"10.0.0.1"}})
	if err == nil || !strings.Contains(err.Error(), "DNS apply") {
		t.Fatalf("expected DNS hook error, got %v", err)
	}
	if step.Kind != "dns" || step.Owner != netexecutor.OwnerDNS {
		t.Fatalf("expected DNS step to be preserved, got %#v", step)
	}
}

type recordingRouteExecutor struct{}

func (recordingRouteExecutor) Add(context.Context, planner.TunRoutePlan) (netexecutor.Step, error) {
	return netexecutor.Step{Kind: "route", Target: "default", Owner: netexecutor.OwnerRoute}, nil
}

func (recordingRouteExecutor) Verify(context.Context, planner.TunRoutePlan) error { return nil }
func (recordingRouteExecutor) Rollback(context.Context, planner.TunRoutePlan) error { return nil }

type recordingDNSExecutor struct{}

func (recordingDNSExecutor) Apply(context.Context, planner.TunDNSPlan) (netexecutor.Step, error) {
	return netexecutor.Step{Kind: "dns", Target: "podlaz0", Owner: netexecutor.OwnerDNS}, nil
}

func (recordingDNSExecutor) Verify(context.Context, planner.TunDNSPlan) error { return nil }
func (recordingDNSExecutor) Rollback(context.Context, planner.TunDNSPlan) error { return nil }
