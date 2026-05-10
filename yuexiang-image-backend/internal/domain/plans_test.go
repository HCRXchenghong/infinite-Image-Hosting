package domain

import "testing"

func TestDefaultPlansHideInfiniteMaxFromPublicPlans(t *testing.T) {
	plans := DefaultPlans()
	public := PublicPlans(plans)

	if len(public) != 4 {
		t.Fatalf("expected 4 public plans, got %d", len(public))
	}
	if _, ok := FindPlan(public, "infinite-max"); ok {
		t.Fatalf("infinite-max must not be public")
	}
}

func TestInfiniteMaxUsesNilQuotasForUnlimitedResources(t *testing.T) {
	plan, ok := FindPlan(DefaultPlans(), "infinite-max")
	if !ok {
		t.Fatalf("missing infinite-max")
	}
	if !plan.Unlimited || !plan.InviteOnly || plan.Purchasable || plan.Visibility != PlanHidden {
		t.Fatalf("invalid infinite-max flags: %+v", plan)
	}
	if plan.Quota.StorageBytes != nil || plan.Quota.BandwidthBytes != nil || plan.Quota.APICalls != nil {
		t.Fatalf("unlimited quotas should be nil: %+v", plan.Quota)
	}
	if !plan.Quota.Allows(plan.Quota.StorageBytes, 1<<62) {
		t.Fatalf("nil quota should allow very large usage")
	}
}
