package data

import (
	"context"
	"strings"
	"testing"
)

type fakeExecutor struct {
	rows []Row
	spec QuerySpec
}

func (f *fakeExecutor) Execute(_ context.Context, spec QuerySpec) ([]Row, error) {
	f.spec = spec
	return f.rows, nil
}

func TestRetrieveBuildsParameterizedQuery(t *testing.T) {
	executor := &fakeExecutor{rows: []Row{{DataType: "exp"}}}
	service := NewService(executor, "project.dataset.table", 1_000)

	output, err := service.Retrieve(context.Background(), RetrieveInput{
		DataTypes: []string{" EXP ", "exp"},
		Datasets:  []string{"CCLE"},
		Limit:     10,
	})
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if output.Count != 1 || output.Limit != 10 {
		t.Fatalf("Retrieve() output = %+v", output)
	}
	if strings.Contains(executor.spec.SQL, "CCLE") || strings.Contains(executor.spec.SQL, "EXP") {
		t.Fatalf("query contains unparameterized values: %s", executor.spec.SQL)
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

func TestNormalizeValues(t *testing.T) {
	values := normalizeValues([]string{" TP53 ", "tp53", "", "MDM2"})
	if len(values) != 2 || values[0] != "tp53" || values[1] != "mdm2" {
		t.Fatalf("normalizeValues() = %#v", values)
	}
}
