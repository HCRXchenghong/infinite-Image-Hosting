package observability

type MetricSnapshot struct {
	Users            int   `json:"users"`
	Images           int   `json:"images"`
	StorageBytes     int64 `json:"storage_bytes"`
	MonthlyBandwidth int64 `json:"monthly_bandwidth_bytes"`
	ImageRequests    int64 `json:"image_requests"`
	APICalls         int64 `json:"api_calls"`
	RiskEvents       int   `json:"risk_events"`
}
