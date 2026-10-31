package data

import (
	"context"
	"strings"
	"testing"
)

type fakeExecutor struct {
	rows    []Row
	metrics QueryMetrics
	spec    QuerySpec
}

func (f *fakeExecutor) Execute(_ context.Context, spec QuerySpec) (ExecutionResult, error) {
	f.spec = spec
	return ExecutionResult{Rows: f.rows, Metrics: f.metrics}, nil
}

func TestRetrieveBuildsParameterizedQuery(t *testing.T) {
	executor := &fakeExecutor{
		rows: []Row{{DataType: "exp"}},
		metrics: QueryMetrics{
			JobID:                    "job-123",
			TotalBytesProcessed:      100,
			TotalBytesBilled:         200,
			TotalSlotMilliseconds:    300,
			EstimatedOnDemandCostUSD: 0.001,
			OnDemandPricePerTiBUSD:   6.25,
		},
	}
	service := NewService(executor, "project.dataset.table", 1_000)

	output, err := service.Retrieve(context.Background(), RetrieveInput{
		DataTypes: []string{" exp ", "exp"},
		Datasets:  []string{"ccle"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if output.Count != 1 || output.Limit != 10 {
		t.Fatalf("Retrieve() output = %+v", output)
	}
	if output.QueryMetrics != executor.metrics {
		t.Fatalf("Retrieve() metrics = %+v, want %+v", output.QueryMetrics, executor.metrics)
	}
	if strings.Contains(executor.spec.SQL, "ccle") || strings.Contains(executor.spec.SQL, "exp") {
		t.Fatalf("query contains unparameterized values: %s", executor.spec.SQL)
	}
	if strings.Contains(executor.spec.SQL, "LOWER(") {
		t.Fatalf("query unexpectedly case-folds a filter: %s", executor.spec.SQL)
	}
	if !strings.Contains(executor.spec.SQL, "data_type IN UNNEST(@data_types)") {
		t.Fatalf("query does not use direct equality filtering: %s", executor.spec.SQL)
	}
	if len(executor.spec.Parameters) != 3 {
		t.Fatalf("parameter count = %d, want 3", len(executor.spec.Parameters))
	}
}

func TestRetrieveCapsRowsAndReportsTruncation(t *testing.T) {
	executor := &fakeExecutor{rows: []Row{{VarName: "A"}, {VarName: "B"}, {VarName: "C"}}}
	service := NewService(executor, "project.dataset.table", 2)

	output, err := service.Retrieve(context.Background(), RetrieveInput{Limit: 50})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if output.Count != 2 || output.Limit != 2 || !output.Truncated {
		t.Fatalf("Retrieve() output = %+v", output)
	}
}

func TestRetrieveRejectsNegativeLimit(t *testing.T) {
	service := NewService(&fakeExecutor{}, "project.dataset.table", 1_000)
	if _, err := service.Retrieve(context.Background(), RetrieveInput{Limit: -1}); err == nil {
		t.Fatal("Retrieve() error = nil, want validation error")
	}
}

func TestCleanValuesPreservesCanonicalCase(t *testing.T) {
	values := cleanValues([]string{" TP53 ", "TP53", "tp53", "", "MDM2"})
	if len(values) != 3 || values[0] != "TP53" || values[1] != "tp53" || values[2] != "MDM2" {
		t.Fatalf("cleanValues() = %#v", values)
	}
}

func TestBuildQueryPreservesMixedCaseCanonicalValues(t *testing.T) {
	spec := BuildQuery("project.dataset.table", RetrieveInput{
		Datasets: []string{"nci60Hts384"},
		ObsNames: []string{"CVCL_0031"},
		VarNames: []string{"TP53"},
		Tissues:  []string{"Soft_Tissue"},
	}, 10)

	if strings.Contains(spec.SQL, "LOWER(") {
		t.Fatalf("BuildQuery() SQL unexpectedly contains LOWER(): %s", spec.SQL)
	}
	wantValues := map[string]string{
		"datasets":  "nci60Hts384",
		"obs_names": "CVCL_0031",
		"var_names": "TP53",
		"tissues":   "Soft_Tissue",
	}
	for _, parameter := range spec.Parameters {
		want, ok := wantValues[parameter.Name]
		if !ok {
			continue
		}
		values, ok := parameter.Value.([]string)
		if !ok || len(values) != 1 || values[0] != want {
			t.Errorf("parameter %s = %#v, want %q", parameter.Name, parameter.Value, want)
		}
		delete(wantValues, parameter.Name)
	}
	if len(wantValues) != 0 {
		t.Fatalf("missing parameters: %#v", wantValues)
	}
}
