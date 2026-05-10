package domain

type PlanVisibility string

const (
	PlanVisible PlanVisibility = "visible"
	PlanHidden  PlanVisibility = "hidden"
)

type Quota struct {
	StorageBytes       *int64 `json:"storage_bytes"`
	BandwidthBytes     *int64 `json:"bandwidth_bytes"`
	ImageRequests      *int64 `json:"image_requests"`
	APICalls           *int64 `json:"api_calls"`
	ImageProcessEvents *int64 `json:"image_process_events"`
	SingleFileBytes    *int64 `json:"single_file_bytes"`
}

type Plan struct {
	Slug             string         `json:"slug"`
	Name             string         `json:"name"`
	MonthlyPriceCent int64          `json:"monthly_price_cent"`
	YearlyPriceCent  int64          `json:"yearly_price_cent"`
	Visibility       PlanVisibility `json:"visibility"`
	Purchasable      bool           `json:"purchasable"`
	InviteOnly       bool           `json:"invite_only"`
	Unlimited        bool           `json:"unlimited"`
	Quota            Quota          `json:"quota"`
}

func DefaultPlans() []Plan {
	return []Plan{
		{
			Slug:             "go",
			Name:             "Go",
			MonthlyPriceCent: 1200,
			YearlyPriceCent:  12000,
			Visibility:       PlanVisible,
			Purchasable:      true,
			Quota: Quota{
				StorageBytes:       ptrInt64(10 * GB),
				BandwidthBytes:     ptrInt64(100 * GB),
				ImageRequests:      ptrInt64(1_000_000),
				APICalls:           ptrInt64(50_000),
				ImageProcessEvents: ptrInt64(3_000),
				SingleFileBytes:    ptrInt64(20 * MB),
			},
		},
		{
			Slug:             "plus",
			Name:             "Plus",
			MonthlyPriceCent: 2900,
			YearlyPriceCent:  29900,
			Visibility:       PlanVisible,
			Purchasable:      true,
			Quota: Quota{
				StorageBytes:       ptrInt64(50 * GB),
				BandwidthBytes:     ptrInt64(500 * GB),
				ImageRequests:      ptrInt64(10_000_000),
				APICalls:           ptrInt64(500_000),
				ImageProcessEvents: ptrInt64(20_000),
				SingleFileBytes:    ptrInt64(50 * MB),
			},
		},
		{
			Slug:             "pro",
			Name:             "Pro",
			MonthlyPriceCent: 7900,
			YearlyPriceCent:  79900,
			Visibility:       PlanVisible,
			Purchasable:      true,
			Quota: Quota{
				StorageBytes:       ptrInt64(200 * GB),
				BandwidthBytes:     ptrInt64(2 * TB),
				ImageRequests:      ptrInt64(50_000_000),
				APICalls:           ptrInt64(5_000_000),
				ImageProcessEvents: ptrInt64(100_000),
				SingleFileBytes:    ptrInt64(100 * MB),
			},
		},
		{
			Slug:             "ultra",
			Name:             "Ultra",
			MonthlyPriceCent: 12000,
			YearlyPriceCent:  120000,
			Visibility:       PlanVisible,
			Purchasable:      true,
			Quota: Quota{
				StorageBytes:       ptrInt64(500 * GB),
				BandwidthBytes:     ptrInt64(5 * TB),
				ImageRequests:      ptrInt64(120_000_000),
				APICalls:           ptrInt64(15_000_000),
				ImageProcessEvents: ptrInt64(300_000),
				SingleFileBytes:    ptrInt64(100 * MB),
			},
		},
		{
			Slug:             "infinite-max",
			Name:             "Infinite Max",
			MonthlyPriceCent: 0,
			YearlyPriceCent:  0,
			Visibility:       PlanHidden,
			Purchasable:      false,
			InviteOnly:       true,
			Unlimited:        true,
			Quota: Quota{
				SingleFileBytes: ptrInt64(500 * MB),
			},
		},
	}
}

func PublicPlans(plans []Plan) []Plan {
	result := make([]Plan, 0, len(plans))
	for _, plan := range plans {
		if plan.Visibility == PlanVisible && plan.Purchasable {
			result = append(result, plan)
		}
	}
	return result
}

func FindPlan(plans []Plan, slug string) (Plan, bool) {
	for _, plan := range plans {
		if plan.Slug == slug {
			return plan, true
		}
	}
	return Plan{}, false
}

func (q Quota) Allows(metric *int64, used int64) bool {
	return metric == nil || used <= *metric
}

func ptrInt64(value int64) *int64 {
	return &value
}

const (
	KB int64 = 1024
	MB       = 1024 * KB
	GB       = 1024 * MB
	TB       = 1024 * GB
)
