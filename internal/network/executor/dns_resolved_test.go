package executor

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/AidarKhusainov/tunwarden/internal/network/planner"
)

func TestResolvedDNSExecutorApplyVerifyAndRollbackCommands(t *testing.T) {
	runner := &recordingRunner{stdout: "Link 7 (tunwarden0)\n    DNS Servers: 1.1.1.1\n    DNS Domain: ~."}
	exec := ResolvedDNSExecutor{Runner: runner}
	plan := dnsPlanForTest()

	step, err := exec.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("apply DNS: %v", err)
	}
	if step.Kind != "dns" || step.Target != "tunwarden0" || step.Owner != OwnerDNS {
		t.Fatalf("unexpected DNS step: %#v", step)
	}
	if err := exec.Verify(context.Background(), plan); err != nil {
		t.Fatalf("verify DNS: %v", err)
	}
	if err := exec.Rollback(context.Background(), plan); err != nil {
		t.Fatalf("rollback DNS: %v", err)
	}

	want := [][]string{
		{"resolvectl", "dns", "tunwarden0", "1.1.1.1"},
		{"resolvectl", "domain", "tunwarden0", "~."},
		{"resolvectl", "default-route", "tunwarden0", "yes"},
		{"resolvectl", "status", "tunwarden0", "--no-pager"},
		{"resolvectl", "revert", "tunwarden0"},
	}
	if !reflect.DeepEqual(runner.commands, want) {
		t.Fatalf("unexpected commands:\nwant %#v\n got %#v", want, runner.commands)
	}
}

func TestResolvedDNSExecutorFailsClearlyWhenPlanIsBlocked(t *testing.T) {
	plan := dnsPlanForTest()
	plan.Action = planner.DNSActionBlocked
	plan.Reason = "systemd-resolved state is missing"

	_, err := (ResolvedDNSExecutor{Runner: &recordingRunner{}}).Apply(context.Background(), plan)
	if err == nil {
		t.Fatal("expected blocked DNS plan failure")
	}
}

func TestResolvedDNSExecutorVerifyRequiresRouteOnlyDomain(t *testing.T) {
	plan := dnsPlanForTest()
	err := (ResolvedDNSExecutor{Runner: &recordingRunner{stdout: "Link 7 (tunwarden0)\n    DNS Servers: 1.1.1.1"}}).Verify(context.Background(), plan)
	if err == nil {
		t.Fatal("expected verify failure when route-only domain is missing")
	}
}

func TestDNSAwareTunExecutorAppliesVerifiesAndRollsBackDNSInSafeOrder(t *testing.T) {
	recorder := &callRecorder{}
	exec := DNSAwareTunExecutor{
		Base: TunExecutor{TunDevice: fakeTun{rec: recorder}, Routes: fakeRoutes{rec: recorder}, PolicyRules: fakeRules{rec: recorder}},
		DNS:  fakeDNS{rec: recorder},
	}
	plan := executorPlanForTest()
	plan.DNS = dnsPlanForTest()

	steps, err := exec.Apply(context.Background(), plan)
	if err != nil {
		t.Fatalf("apply DNS-aware plan: %v", err)
	}
	if len(steps) != 6 {
		t.Fatalf("expected TUN, route, policy-rule, and DNS steps, got %#v", steps)
	}
	if err := exec.Verify(context.Background(), plan); err != nil {
		t.Fatalf("verify DNS-aware plan: %v", err)
	}
	if err := exec.Rollback(context.Background(), plan); err != nil {
		t.Fatalf("rollback DNS-aware plan: %v", err)
	}

	want := []string{
		"tun:create:tunwarden0",
		"route:add:tunwarden:default",
		"route:add:main:203.0.113.10/32",
		"rule:add:51819:to 203.0.113.10/32",
		"rule:add:51820:from all",
		"dns:apply:tunwarden0",
		"tun:verify:tunwarden0",
		"route:verify:tunwarden:default",
		"route:verify:main:203.0.113.10/32",
		"rule:verify:51819:to 203.0.113.10/32",
		"rule:verify:51820:from all",
		"dns:verify:tunwarden0",
		"dns:rollback:tunwarden0",
		"rule:rollback:51820:from all",
		"rule:rollback:51819:to 203.0.113.10/32",
		"route:rollback:main:203.0.113.10/32",
		"route:rollback:tunwarden:default",
		"tun:rollback:tunwarden0",
	}
	if !reflect.DeepEqual(recorder.calls, want) {
		t.Fatalf("unexpected calls:\nwant %#v\n got %#v", want, recorder.calls)
	}
}

func TestDNSAwareTunExecutorRollsBackDNSWhenApplyFailsAfterDNSMutation(t *testing.T) {
	recorder := &callRecorder{}
	exec := DNSAwareTunExecutor{
		Base: TunExecutor{TunDevice: fakeTun{rec: recorder}, Routes: fakeRoutes{rec: recorder}, PolicyRules: fakeRules{rec: recorder}},
		DNS:  fakeDNS{rec: recorder, applyErr: errors.New("resolved failure")},
	}
	plan := executorPlanForTest()
	plan.DNS = dnsPlanForTest()

	steps, err := exec.Apply(context.Background(), plan)
	if err == nil {
		t.Fatal("expected DNS apply failure")
	}
	if len(steps) != 5 {
		t.Fatalf("expected base networking steps to remain rollbackable, got %#v", steps)
	}
	if recorder.calls[len(recorder.calls)-1] != "dns:rollback:tunwarden0" {
		t.Fatalf("expected immediate DNS rollback after DNS apply failure, got %#v", recorder.calls)
	}
}

func dnsPlanForTest() planner.TunDNSPlan {
	return planner.TunDNSPlan{
		Backend:    planner.DNSBackendSystemdResolved,
		TargetLink: "tunwarden0",
		Action:     planner.DNSActionConfigure,
		Reason:     "use systemd-resolved per-link DNS",
	}
}

type fakeDNS struct {
	rec      *callRecorder
	applyErr error
}

func (f fakeDNS) Apply(_ context.Context, plan planner.TunDNSPlan) (Step, error) {
	f.rec.calls = append(f.rec.calls, "dns:apply:"+plan.TargetLink)
	if f.applyErr != nil {
		return Step{}, f.applyErr
	}
	return Step{Kind: "dns", Target: plan.TargetLink, Owner: OwnerDNS}, nil
}

func (f fakeDNS) Verify(_ context.Context, plan planner.TunDNSPlan) error {
	f.rec.calls = append(f.rec.calls, "dns:verify:"+plan.TargetLink)
	return nil
}

func (f fakeDNS) Rollback(_ context.Context, plan planner.TunDNSPlan) error {
	f.rec.calls = append(f.rec.calls, "dns:rollback:"+plan.TargetLink)
	return nil
}
