// Package data builds bounded, parameterized CellMinerCDB retrieval queries.
package data

import (
	"context"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"
)

const (
	defaultRequestLimit = 100
	maxFilterValues     = 100
)

// RetrieveInput contains optional exact-match filters for a data query.
type RetrieveInput struct {
	DataTypes []string `json:"data_types,omitempty" jsonschema:"Case-sensitive canonical lowercase modality codes: act, cop, mut, met, mth, bmt, rrb, exp, xai, pro, mir, mda, swa, xsq, cri, mtb, his, hs4, or sur"`
	Datasets  []string `json:"datasets,omitempty" jsonschema:"Case-sensitive canonical dataset codes: acc, achilles, almanac, ccle, ctrp, gdsc1, gdsc2, gdscDec15, mdaMills, nci60, nci60Hts384, nciSarcoma, nciSclc, prism, tncats, or uniSarcoma"`
	ObsNames  []string `json:"obs_names,omitempty" jsonschema:"Case-sensitive canonical Cellosaurus accessions beginning with uppercase CVCL_. Resolve synonyms first; for example, MCF7 or MCF-7 maps to CVCL_0031"`
	VarNames  []string `json:"var_names,omitempty" jsonschema:"Case-sensitive canonical identifiers: approved uppercase HGNC symbols for human genes, canonical protein or microRNA identifiers for their modality, or numeric PubChem CIDs for compounds. Resolve synonyms first; for example, p53 maps to TP53 and HER2 maps to ERBB2"`
	Tissues   []string `json:"tissues,omitempty" jsonschema:"Case-sensitive canonical tissue labels: Adrenal_Gland, Biliary_Tract, Bladder_Urinary_Tract, Blood, Bone, Bowel, Brain_CNS, Breast, Cervix, Esophagus_Stomach, Eye, Head_Neck, Kidney, Liver, Lung, Lymph, Mesothelium, Other, Ovary, Pancreas, Peripheral_Nervous_System, Prostate, Skin, Soft_Tissue, Thyroid, Uterus, or Vulva"`
	Limit     int      `json:"limit,omitempty" jsonschema:"Maximum rows to return; defaults to 100 and is capped by the server. Check truncated to determine whether the response is incomplete"`
}

// Row is one CellMinerCDB long-format data record.
type Row struct {
	DataType string  `json:"data_type"`
	Dataset  string  `json:"dataset"`
	ObsName  string  `json:"obs_name"`
	VarName  string  `json:"var_name"`
	Value    float64 `json:"value"`
	Tissue   *string `json:"tissue"`
}

// RetrieveOutput is the structured MCP response.
type RetrieveOutput struct {
	Rows         []Row        `json:"rows"`
	Count        int          `json:"count"`
	Limit        int          `json:"limit"`
	Truncated    bool         `json:"truncated"`
	QueryMetrics QueryMetrics `json:"query_metrics"`
}

// QueryMetrics reports actual BigQuery job statistics after execution.
type QueryMetrics struct {
	JobID                    string  `json:"job_id"`
	CacheHit                 bool    `json:"cache_hit"`
	TotalBytesProcessed      int64   `json:"total_bytes_processed"`
	TotalBytesBilled         int64   `json:"total_bytes_billed"`
	TotalSlotMilliseconds    int64   `json:"total_slot_ms"`
	EstimatedOnDemandCostUSD float64 `json:"estimated_on_demand_cost_usd"`
	OnDemandPricePerTiBUSD   float64 `json:"on_demand_price_per_tib_usd"`
}

// ExecutionResult contains retrieved rows and their BigQuery job metrics.
type ExecutionResult struct {
	Rows    []Row
	Metrics QueryMetrics
}

// QuerySpec is a complete parameterized query request.
type QuerySpec struct {
	SQL        string
	Parameters []bigquery.QueryParameter
}

// Executor executes a parameterized retrieval query.
type Executor interface {
	Execute(context.Context, QuerySpec) (ExecutionResult, error)
}

// Service validates retrieval input and coordinates query execution.
type Service struct {
	executor Executor
	tableFQN string
	maxRows  int
}

// NewService creates a bounded data retrieval service.
//
// Args:
//   - executor: Query backend.
//   - tableFQN: Validated project.dataset.table identifier.
//   - maxRows: Server-side maximum rows returned per call.
//
// Returns:
//   - *Service: Configured retrieval service.
func NewService(executor Executor, tableFQN string, maxRows int) *Service {
	return &Service{executor: executor, tableFQN: tableFQN, maxRows: maxRows}
}

// Retrieve executes a safe query and reports whether more matching rows exist.
//
// Args:
//   - ctx: Query cancellation context.
//   - input: Optional exact-match filters and requested limit.
//
// Returns:
//   - RetrieveOutput: JSON-ready rows and pagination metadata.
//   - error: Validation or BigQuery execution error.
func (s *Service) Retrieve(ctx context.Context, input RetrieveInput) (RetrieveOutput, error) {
	limit, err := s.effectiveLimit(input.Limit)
	if err != nil {
		return RetrieveOutput{}, err
	}
	if err := validateFilterSizes(input); err != nil {
		return RetrieveOutput{}, err
	}

	spec := BuildQuery(s.tableFQN, input, limit+1)
	execution, err := s.executor.Execute(ctx, spec)
	if err != nil {
		return RetrieveOutput{}, fmt.Errorf("retrieve BigQuery rows: %w", err)
	}
	rows := execution.Rows

	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	if rows == nil {
		rows = []Row{}
	}

	return RetrieveOutput{
		Rows:         rows,
		Count:        len(rows),
		Limit:        limit,
		Truncated:    truncated,
		QueryMetrics: execution.Metrics,
	}, nil
}

func (s *Service) effectiveLimit(requested int) (int, error) {
	if requested < 0 {
		return 0, fmt.Errorf("limit cannot be negative")
	}
	if requested == 0 {
		requested = defaultRequestLimit
	}
	if requested > s.maxRows {
		requested = s.maxRows
	}
	return requested, nil
}

func validateFilterSizes(input RetrieveInput) error {
	filters := map[string][]string{
		"data_types": input.DataTypes,
		"datasets":   input.Datasets,
		"obs_names":  input.ObsNames,
		"var_names":  input.VarNames,
		"tissues":    input.Tissues,
	}
	for name, values := range filters {
		if len(values) > maxFilterValues {
			return fmt.Errorf("%s accepts at most %d values", name, maxFilterValues)
		}
	}
	return nil
}

// BuildQuery creates a parameterized BigQuery retrieval query.
//
// Args:
//   - tableFQN: Validated project.dataset.table identifier.
//   - input: Exact-match filters.
//   - scanLimit: Maximum rows requested from BigQuery.
//
// Returns:
//   - QuerySpec: SQL and named parameters.
func BuildQuery(tableFQN string, input RetrieveInput, scanLimit int) QuerySpec {
	clauses := []string{"value IS NOT NULL"}
	parameters := make([]bigquery.QueryParameter, 0, 6)

	addFilter := func(column string, parameter string, values []string) {
		cleaned := cleanValues(values)
		if len(cleaned) == 0 {
			return
		}
		clauses = append(
			clauses,
			fmt.Sprintf("%s IN UNNEST(@%s)", column, parameter),
		)
		parameters = append(parameters, bigquery.QueryParameter{Name: parameter, Value: cleaned})
	}

	addFilter("data_type", "data_types", input.DataTypes)
	addFilter("dataset", "datasets", input.Datasets)
	addFilter("obs_name", "obs_names", input.ObsNames)
	addFilter("var_name", "var_names", input.VarNames)
	addFilter("tissue", "tissues", input.Tissues)
	parameters = append(parameters, bigquery.QueryParameter{Name: "scan_limit", Value: scanLimit})

	sql := fmt.Sprintf(
		"SELECT data_type, dataset, obs_name, var_name, value, tissue\n"+
			"FROM `%s`\n"+
			"WHERE %s\n"+
			"LIMIT @scan_limit",
		tableFQN,
		strings.Join(clauses, "\n  AND "),
	)
	return QuerySpec{SQL: sql, Parameters: parameters}
}

func cleanValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		cleaned = append(cleaned, value)
	}
	return cleaned
}
