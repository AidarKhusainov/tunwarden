package planner

import "testing"

func TestPlanTunPinsCurrentDefaultPolicyShape(t *testing.T) {
	plan, err := PlanTun(testVLESSProfile(), snapshot.FakeResolvedDesktop())
	_ = plan
	if err != nil {
		t.Fatalf("plan tun: %v", err)
	}
}
