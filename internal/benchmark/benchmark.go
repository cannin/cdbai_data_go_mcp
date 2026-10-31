// Package benchmark measures retrieval latency without retaining returned data.
package benchmark

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// Result records one retrieval trial.
type Result struct {
	Backend        string  `json:"backend"`
	Run            int     `json:"run"`
	RequestedRows  int     `json:"requested_rows"`
	ReturnedRows   int     `json:"returned_rows"`
	DurationMS     float64 `json:"duration_ms"`
	BytesProcessed int64   `json:"bytes_processed,omitempty"`
	BytesBilled    int64   `json:"bytes_billed,omitempty"`
	CacheHit       bool    `json:"cache_hit"`
}

// Summary contains aggregate latency and BigQuery processing measurements.
type Summary struct {
	Runs                int     `json:"runs"`
	MinDurationMS       float64 `json:"min_duration_ms"`
	MeanDurationMS      float64 `json:"mean_duration_ms"`
	MedianDurationMS    float64 `json:"median_duration_ms"`
	P95DurationMS       float64 `json:"p95_duration_ms"`
	TotalBytesProcessed int64   `json:"total_bytes_processed"`
	TotalBytesBilled    int64   `json:"total_bytes_billed"`
}

// Report is a benchmark artifact containing configuration-free measurements.
type Report struct {
	GeneratedAt time.Time `json:"generated_at"`
	Backend     string    `json:"backend"`
	DataType    string    `json:"data_type"`
	Dataset     string    `json:"dataset"`
	Results     []Result  `json:"results"`
	Summary     Summary   `json:"summary"`
}

// Runner executes one retrieval amount against a database backend.
type Runner interface {
	Name() string
	Run(context.Context, int) (Result, error)
}

// Execute runs each requested row amount once and summarizes timings.
//
// Args:
//   - ctx: Benchmark cancellation context.
//   - runner: Configured database backend.
//   - limits: Requested row amounts, in execution order.
//   - dataType: Modality filter recorded in the report.
//   - dataset: Dataset filter recorded in the report.
//
// Returns:
//   - Report: Completed benchmark results.
//   - error: First failed trial.
func Execute(
	ctx context.Context,
	runner Runner,
	limits []int,
	dataType string,
	dataset string,
) (Report, error) {
	results := make([]Result, 0, len(limits))
	for index, limit := range limits {
		result, err := runner.Run(ctx, limit)
		if err != nil {
			return Report{}, fmt.Errorf("%s trial %d with limit %d: %w", runner.Name(), index+1, limit, err)
		}
		result.Backend = runner.Name()
		result.Run = index + 1
		results = append(results, result)
	}

	return Report{
		GeneratedAt: time.Now().UTC(),
		Backend:     runner.Name(),
		DataType:    dataType,
		Dataset:     dataset,
		Results:     results,
		Summary:     summarize(results),
	}, nil
}

// WriteReport writes JSON and CSV benchmark artifacts.
//
// Args:
//   - outputDir: Destination directory.
//   - report: Completed measurements.
//
// Returns:
//   - string: JSON artifact path.
//   - string: CSV artifact path.
//   - error: Directory or serialization error.
func WriteReport(outputDir string, report Report) (string, string, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create output directory: %w", err)
	}
	stamp := report.GeneratedAt.Format("20060102T150405Z")
	base := fmt.Sprintf("%s-%s", report.Backend, stamp)
	jsonPath := filepath.Join(outputDir, base+".json")
	csvPath := filepath.Join(outputDir, base+".csv")

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal JSON report: %w", err)
	}
	if err := os.WriteFile(jsonPath, append(jsonData, '\n'), 0o644); err != nil {
		return "", "", fmt.Errorf("write JSON report: %w", err)
	}

	csvFile, err := os.Create(csvPath)
	if err != nil {
		return "", "", fmt.Errorf("create CSV report: %w", err)
	}
	writer := csv.NewWriter(csvFile)
	records := [][]string{{
		"backend", "run", "requested_rows", "returned_rows", "duration_ms",
		"bytes_processed", "bytes_billed", "cache_hit",
	}}
	for _, result := range report.Results {
		records = append(records, []string{
			result.Backend,
			strconv.Itoa(result.Run),
			strconv.Itoa(result.RequestedRows),
			strconv.Itoa(result.ReturnedRows),
			strconv.FormatFloat(result.DurationMS, 'f', 3, 64),
			strconv.FormatInt(result.BytesProcessed, 10),
			strconv.FormatInt(result.BytesBilled, 10),
			strconv.FormatBool(result.CacheHit),
		})
	}
	if err := writer.WriteAll(records); err != nil {
		csvFile.Close()
		return "", "", fmt.Errorf("write CSV report: %w", err)
	}
	if err := csvFile.Close(); err != nil {
		return "", "", fmt.Errorf("close CSV report: %w", err)
	}
	return jsonPath, csvPath, nil
}

func summarize(results []Result) Summary {
	durations := make([]float64, len(results))
	var totalDuration float64
	var bytesProcessed int64
	var bytesBilled int64
	for index, result := range results {
		durations[index] = result.DurationMS
		totalDuration += result.DurationMS
		bytesProcessed += result.BytesProcessed
		bytesBilled += result.BytesBilled
	}
	if len(durations) == 0 {
		return Summary{}
	}
	sort.Float64s(durations)
	median := durations[len(durations)/2]
	if len(durations)%2 == 0 {
		median = (durations[len(durations)/2-1] + median) / 2
	}
	p95Index := int(math.Ceil(float64(len(durations))*0.95)) - 1
	return Summary{
		Runs:                len(results),
		MinDurationMS:       durations[0],
		MeanDurationMS:      totalDuration / float64(len(results)),
		MedianDurationMS:    median,
		P95DurationMS:       durations[p95Index],
		TotalBytesProcessed: bytesProcessed,
		TotalBytesBilled:    bytesBilled,
	}
}
