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
	DataTypes []string `json:"data_types,omitempty" jsonschema:"CellMinerCDB modality codes such as act, exp, xsq, mut, cop, or pro"`
	Datasets  []string `json:"datasets,omitempty" jsonschema:"Dataset codes such as ccle, ctrp, gdsc1, nci60, or prism"`
	ObsNames  []string `json:"obs_names,omitempty" jsonschema:"Cellosaurus cell-line identifiers beginning with CVCL_"`
	VarNames  []string `json:"var_names,omitempty" jsonschema:"HGNC gene symbols, protein identifiers, microRNA identifiers, or numeric PubChem IDs"`
	Tissues   []string `json:"tissues,omitempty" jsonschema:"Tissue names such as Blood, Breast, Lung, or Skin"`
	Limit     int      `json:"limit,omitempty" jsonschema:"Maximum rows to return; defaults to 100 and is capped by the server"`
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
	Rows      []Row `json:"rows"`
	Count     int   `json:"count"`
	Limit     int   `json:"limit"`
	Truncated bool  `json:"truncated"`
}

// QuerySpec is a complete parameterized query request.
type QuerySpec struct {
	SQL        string
	Parameters []bigquery.QueryParameter
}

// Executor executes a parameterized retrieval query.
type Executor interface {
	Execute(context.Context, QuerySpec) ([]Row, error)
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

	spec := buildQuery(s.tableFQN, input, limit+1)
	rows, err := s.executor.Execute(ctx, spec)
	if err != nil {
		return RetrieveOutput{}, fmt.Errorf("retrieve BigQuery rows: %w", err)
	}

	truncated := len(rows) > limit
	if truncated {
		rows = rows[:limit]
	}
	if rows == nil {
		rows = []Row{}
	}

	return RetrieveOutput{
		Rows:      rows,
		Count:     len(rows),
		Limit:     limit,
		Truncated: truncated,
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

func buildQuery(tableFQN string, input RetrieveInput, scanLimit int) QuerySpec {
	clauses := []string{"value IS NOT NULL"}
	parameters := make([]bigquery.QueryParameter, 0, 6)

	addFilter := func(column string, parameter string, values []string) {
		normalized := normalizeValues(values)
		if len(normalized) == 0 {
			return
		}
		clauses = append(
			clauses,
			fmt.Sprintf("LOWER(%s) IN UNNEST(@%s)", column, parameter),
		)
		parameters = append(parameters, bigquery.QueryParameter{Name: parameter, Value: normalized})
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

func normalizeValues(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	return normalized
}
