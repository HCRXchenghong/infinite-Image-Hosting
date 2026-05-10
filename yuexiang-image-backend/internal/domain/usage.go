package domain

type Usage struct {
	StorageBytes       int64 `json:"storage_bytes"`
	BandwidthBytes     int64 `json:"bandwidth_bytes"`
	ImageRequests      int64 `json:"image_requests"`
	APICalls           int64 `json:"api_calls"`
	ImageProcessEvents int64 `json:"image_process_events"`
}

type QuotaCheck struct {
	Allowed bool   `json:"allowed"`
	Metric  string `json:"metric,omitempty"`
	Limit   *int64 `json:"limit,omitempty"`
	Used    int64  `json:"used,omitempty"`
	Message string `json:"message,omitempty"`
}

func CheckUsage(plan Plan, usage Usage) QuotaCheck {
	if !plan.Quota.Allows(plan.Quota.StorageBytes, usage.StorageBytes) {
		return quotaDenied("storage_bytes", plan.Quota.StorageBytes, usage.StorageBytes)
	}
	if !plan.Quota.Allows(plan.Quota.BandwidthBytes, usage.BandwidthBytes) {
		return quotaDenied("bandwidth_bytes", plan.Quota.BandwidthBytes, usage.BandwidthBytes)
	}
	if !plan.Quota.Allows(plan.Quota.ImageRequests, usage.ImageRequests) {
		return quotaDenied("image_requests", plan.Quota.ImageRequests, usage.ImageRequests)
	}
	if !plan.Quota.Allows(plan.Quota.APICalls, usage.APICalls) {
		return quotaDenied("api_calls", plan.Quota.APICalls, usage.APICalls)
	}
	if !plan.Quota.Allows(plan.Quota.ImageProcessEvents, usage.ImageProcessEvents) {
		return quotaDenied("image_process_events", plan.Quota.ImageProcessEvents, usage.ImageProcessEvents)
	}
	return QuotaCheck{Allowed: true}
}

func CheckUpload(plan Plan, usage Usage, uploadBytes int64) QuotaCheck {
	if plan.Quota.SingleFileBytes != nil && uploadBytes > *plan.Quota.SingleFileBytes {
		return quotaDenied("single_file_bytes", plan.Quota.SingleFileBytes, uploadBytes)
	}
	usage.StorageBytes += uploadBytes
	usage.ImageProcessEvents++
	return CheckUsage(plan, usage)
}

func quotaDenied(metric string, limit *int64, used int64) QuotaCheck {
	return QuotaCheck{
		Allowed: false,
		Metric:  metric,
		Limit:   limit,
		Used:    used,
		Message: "套餐额度不足或触发公平使用限制",
	}
}
