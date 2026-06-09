package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	netexecutor "github.com/AidarKhusainov/tunwarden/internal/network/executor"
	"github.com/AidarKhusainov/tunwarden/internal/network/planner"
	"github.com/AidarKhusainov/tunwarden/internal/profile"
	txstate "github.com/AidarKhusainov/tunwarden/internal/state"
)

func TestTunTransactionWaitsForExplicitCommitAfterApplyAndVerify(t *testing.T) {
	runtimeDir := t.TempDir()
	executor := &recordingTunExecutor{}
	result, err := runTunTransaction(context.Background(), runtimeDir, profile.Profile{ID: "test-profile"}, transactionPlanForTest(), executor, fixedClock())
	if err != nil {
		t.Fatalf("run TUN transaction failed: %v", err)
	}
	tx, _, err := (txstate.TransactionStore{RuntimeDir: runtimeDir}).Load(result.TransactionID)
	if err != nil {
		t.Fatalf("load transaction: %v", err)
	}
	if tx.State != txstate.TransactionVerifying {
		t.Fatalf("expected verifying before core verification, got %s", tx.State)
	}
	if strings.Join(executor.calls, ",") != "apply,verify" {
		t.Fatalf("unexpected executor calls: %#v", executor.calls)
	}
	if err := commitTunTransaction(result.Store, result.TransactionID); err != nil {
		t.Fatalf("commit transaction: %v", err)
	}
	tx, _, err = (txstate.TransactionStore{RuntimeDir: runtimeDir}).Load(result.TransactionID)
	if err != nil {
		t.Fatalf("reload transaction: %v", err)
	}
	if tx.State != txstate.TransactionCommitted {
		t.Fatalf("expected committed transaction after explicit commit, got %s", tx.State)
	}
}

type recordingTunExecutor struct {
	calls []string
}

func (e *recordingTunExecutor) Apply(context.Context, planner.TunPlan) ([]netexecutor.Step, error) {
	e.calls = append(e.calls, "apply")
	return []netexecutor.Step{
		{Kind: "tun-device", Target: "tunwarden0", Owner: netexecutor.OwnerTunDevice},
		{Kind: "route", Target: "tunwarden default", Owner: netexecutor.OwnerRoute},
		{Kind: "policy-rule", Target: "priority 51820 from all lookup tunwarden", Owner: netexecutor.OwnerPolicyRule},
	}, nil
}

func (e *recordingTunExecutor) Verify(context.Context, planner.TunPlan) error {
	e.calls = append(e.calls, "verify")
	return nil
}

func (e *recordingTunExecutor) Rollback(context.Context, planner.TunPlan) error {
	e.calls = append(e.calls, "rollback")
	return nil
}

func transactionPlanForTest() planner.TunPlan {
	return planner.TunPlan{
		ProfileID: "test-profile",
		Mode:      planner.ModeTun,
		TunDevice: planner.TunDevicePlan{Name: "tunwarden0", MTU: 1500, Action: "create"},
		Routes: []planner.TunRoutePlan{{
			Family:      "ipv4",
			Destination: "default",
			Table:       planner.TunRoutingTable,
			Interface:   "tunwarden0",
			Action:      "add",
		}},
		PolicyRules: []planner.TunPolicyRulePlan{{
			Family:   "ipv4",
			Priority: planner.TunRulePriority,
			Selector: planner.IPv4DefaultSelector,
			Table:    planner.TunRoutingTable,
			Action:   "add",
		}},
		Steps: []string{"Plan TUN interface tunwarden0"},
	}
}

func fixedClock() func() time.Time {
	current := time.Date(2026, 6, 8, 12, 0, 0, 0, time.UTC)
	return func() time.Time {
		current = current.Add(time.Millisecond)
		return current
	}
}
