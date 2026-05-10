package domain

import "testing"

func TestCheckUploadEnforcesSingleFileLimit(t *testing.T) {
	plan, _ := FindPlan(DefaultPlans(), "go")
	result := CheckUpload(plan, Usage{}, 21*MB)
	if result.Allowed {
		t.Fatalf("expected upload to be denied")
	}
	if result.Metric != "single_file_bytes" {
		t.Fatalf("expected single file denial, got %s", result.Metric)
	}
}

func TestCheckUploadAllowsInfiniteMaxLargeUsage(t *testing.T) {
	plan, _ := FindPlan(DefaultPlans(), "infinite-max")
	result := CheckUpload(plan, Usage{StorageBytes: 10 * TB, APICalls: 999_999_999}, 200*MB)
	if !result.Allowed {
		t.Fatalf("expected infinite max to allow usage, got %+v", result)
	}
}
