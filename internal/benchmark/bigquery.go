package benchmark

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/cannin/cdbai_data_go_mcp/internal/data"
	"google.golang.org/api/iterator"
)

// BigQueryRunner executes uncached BigQuery retrieval trials.
type BigQueryRunner struct {
	client         *bigquery.Client
	tableFQN       string
	maxBytesBilled int64
	input          data.RetrieveInput
}

// NewBigQueryRunner creates a benchmark backend with fixed filters.
func NewBigQueryRunner(
	client *bigquery.Client,
	tableFQN string,
	maxBytesBilled int64,
	input data.RetrieveInput,
) *BigQueryRunner {
	return &BigQueryRunner{
		client:         client,
		tableFQN:       tableFQN,
		maxBytesBilled: maxBytesBilled,
		input:          input,
	}
}

// Name returns the backend identifier used in reports.
func (r *BigQueryRunner) Name() string {
	return "bigquery"
}

// Run executes one uncached BigQuery retrieval and captures job statistics.
func (r *BigQueryRunner) Run(ctx context.Context, limit int) (Result, error) {
	spec := data.BuildQuery(r.tableFQN, r.input, limit)
	query := r.client.Query(spec.SQL)
	query.UseLegacySQL = false
	query.DisableQueryCache = true
	query.MaxBytesBilled = r.maxBytesBilled
	query.Parameters = spec.Parameters

	started := time.Now()
	job, err := query.Run(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("start query: %w", err)
	}
	rows, err := job.Read(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read query: %w", err)
	}

	rowCount := 0
	for {
		var values []bigquery.Value
		err := rows.Next(&values)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return Result{}, fmt.Errorf("iterate query: %w", err)
		}
		rowCount++
	}
	duration := time.Since(started)

	status, err := job.Status(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("read query statistics: %w", err)
	}
	result := Result{
		RequestedRows: limit,
		ReturnedRows:  rowCount,
		DurationMS:    float64(duration.Microseconds()) / 1_000,
	}
	if status.Statistics != nil {
		result.BytesProcessed = status.Statistics.TotalBytesProcessed
		if details, ok := status.Statistics.Details.(*bigquery.QueryStatistics); ok {
			result.BytesBilled = details.TotalBytesBilled
			result.CacheHit = details.CacheHit
		}
	}
	return result, nil
}
