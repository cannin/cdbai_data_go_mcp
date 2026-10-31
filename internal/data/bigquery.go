package data

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

const (
	bytesPerTiB            = 1 << 40
	onDemandPricePerTiBUSD = 6.25
)

// BigQueryExecutor runs QuerySpec values with a billed-byte ceiling.
type BigQueryExecutor struct {
	client         *bigquery.Client
	maxBytesBilled int64
}

type bigQueryRow struct {
	DataType bigquery.NullString  `bigquery:"data_type"`
	Dataset  bigquery.NullString  `bigquery:"dataset"`
	ObsName  bigquery.NullString  `bigquery:"obs_name"`
	VarName  bigquery.NullString  `bigquery:"var_name"`
	Value    bigquery.NullFloat64 `bigquery:"value"`
	Tissue   bigquery.NullString  `bigquery:"tissue"`
}

// NewBigQueryExecutor creates a BigQuery query backend.
//
// Args:
//   - client: Authenticated BigQuery client.
//   - maxBytesBilled: Maximum billed bytes allowed for each query.
//
// Returns:
//   - *BigQueryExecutor: Configured query backend.
func NewBigQueryExecutor(client *bigquery.Client, maxBytesBilled int64) *BigQueryExecutor {
	return &BigQueryExecutor{client: client, maxBytesBilled: maxBytesBilled}
}

// Execute runs a standard-SQL query and converts rows into the public schema.
//
// Args:
//   - ctx: Query cancellation context.
//   - spec: SQL and named parameters.
//
// Returns:
//   - ExecutionResult: Retrieved data records and actual job metrics.
//   - error: BigQuery submission or iteration error.
func (e *BigQueryExecutor) Execute(ctx context.Context, spec QuerySpec) (ExecutionResult, error) {
	query := e.client.Query(spec.SQL)
	query.UseLegacySQL = false
	query.MaxBytesBilled = e.maxBytesBilled
	query.Parameters = spec.Parameters

	job, err := query.Run(ctx)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("start query: %w", err)
	}
	iteratorRows, err := job.Read(ctx)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read query results: %w", err)
	}

	rows := make([]Row, 0)
	for {
		var raw bigQueryRow
		err := iteratorRows.Next(&raw)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return ExecutionResult{}, fmt.Errorf("read query row: %w", err)
		}
		rows = append(rows, convertRow(raw))
	}

	status, err := job.Status(ctx)
	if err != nil {
		return ExecutionResult{}, fmt.Errorf("read query statistics: %w", err)
	}
	metrics, err := buildQueryMetrics(job.ID(), status)
	if err != nil {
		return ExecutionResult{}, err
	}
	return ExecutionResult{Rows: rows, Metrics: metrics}, nil
}

func buildQueryMetrics(jobID string, status *bigquery.JobStatus) (QueryMetrics, error) {
	if status == nil || status.Statistics == nil {
		return QueryMetrics{}, fmt.Errorf("query statistics are unavailable")
	}
	queryStatistics, ok := status.Statistics.Details.(*bigquery.QueryStatistics)
	if !ok || queryStatistics == nil {
		return QueryMetrics{}, fmt.Errorf("query-specific statistics are unavailable")
	}
	bytesBilled := queryStatistics.TotalBytesBilled
	return QueryMetrics{
		JobID:                    jobID,
		CacheHit:                 queryStatistics.CacheHit,
		TotalBytesProcessed:      status.Statistics.TotalBytesProcessed,
		TotalBytesBilled:         bytesBilled,
		TotalSlotMilliseconds:    queryStatistics.SlotMillis,
		EstimatedOnDemandCostUSD: float64(bytesBilled) / float64(bytesPerTiB) * onDemandPricePerTiBUSD,
		OnDemandPricePerTiBUSD:   onDemandPricePerTiBUSD,
	}, nil
}

func convertRow(raw bigQueryRow) Row {
	row := Row{
		DataType: raw.DataType.StringVal,
		Dataset:  raw.Dataset.StringVal,
		ObsName:  raw.ObsName.StringVal,
		VarName:  raw.VarName.StringVal,
		Value:    raw.Value.Float64,
	}
	if raw.Tissue.Valid {
		tissue := raw.Tissue.StringVal
		row.Tissue = &tissue
	}
	return row
}
