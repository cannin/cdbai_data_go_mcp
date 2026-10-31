package data

import (
	"math"
	"testing"

	"cloud.google.com/go/bigquery"
)

func TestBuildQueryMetrics(t *testing.T) {
	status := &bigquery.JobStatus{
		Statistics: &bigquery.JobStatistics{
			TotalBytesProcessed: 1_000,
			Details: &bigquery.QueryStatistics{
				CacheHit:         true,
				TotalBytesBilled: 2_000,
				SlotMillis:       300,
			},
		},
	}

	metrics, err := buildQueryMetrics("job-123", status)
	if err != nil {
		t.Fatalf("buildQueryMetrics() error = %v", err)
	}
	if metrics.JobID != "job-123" || !metrics.CacheHit {
		t.Fatalf("buildQueryMetrics() identity = %+v", metrics)
	}
	if metrics.TotalBytesProcessed != 1_000 || metrics.TotalBytesBilled != 2_000 {
		t.Fatalf("buildQueryMetrics() bytes = %+v", metrics)
	}
	if metrics.TotalSlotMilliseconds != 300 {
		t.Fatalf("buildQueryMetrics() slot milliseconds = %d", metrics.TotalSlotMilliseconds)
	}
	wantCost := float64(2_000) / float64(bytesPerTiB) * onDemandPricePerTiBUSD
	if math.Abs(metrics.EstimatedOnDemandCostUSD-wantCost) > 1e-15 {
		t.Fatalf(
			"buildQueryMetrics() cost = %.15f, want %.15f",
			metrics.EstimatedOnDemandCostUSD,
			wantCost,
		)
	}
	if metrics.OnDemandPricePerTiBUSD != onDemandPricePerTiBUSD {
		t.Fatalf(
			"buildQueryMetrics() price = %f, want %f",
			metrics.OnDemandPricePerTiBUSD,
			onDemandPricePerTiBUSD,
		)
	}
}

func TestBuildQueryMetricsRejectsMissingStatistics(t *testing.T) {
	if _, err := buildQueryMetrics("job-123", nil); err == nil {
		t.Fatal("buildQueryMetrics() error = nil, want missing statistics error")
	}
	status := &bigquery.JobStatus{Statistics: &bigquery.JobStatistics{}}
	if _, err := buildQueryMetrics("job-123", status); err == nil {
		t.Fatal("buildQueryMetrics() error = nil, want missing query statistics error")
	}
}
