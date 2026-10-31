package benchmark

import (
	"context"
	"testing"
)

type fakeRunner struct{}

func (fakeRunner) Name() string {
	return "fake"
}

func (fakeRunner) Run(_ context.Context, limit int) (Result, error) {
	return Result{
		RequestedRows:  limit,
		ReturnedRows:   limit,
		DurationMS:     float64(limit),
		BytesProcessed: int64(limit * 10),
		BytesBilled:    int64(limit * 20),
	}, nil
}

func TestExecuteSummarizesResults(t *testing.T) {
	report, err := Execute(context.Background(), fakeRunner{}, []int{1, 2, 3, 4}, "cop", "ctrp")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if report.Summary.Runs != 4 || report.Summary.MedianDurationMS != 2.5 {
		t.Fatalf("summary = %+v", report.Summary)
	}
	if report.Summary.TotalBytesProcessed != 100 || report.Summary.TotalBytesBilled != 200 {
		t.Fatalf("summary bytes = %+v", report.Summary)
	}
}

func TestQuotePostgresIdentifier(t *testing.T) {
	quoted, err := quotePostgresIdentifier("public.data")
	if err != nil {
		t.Fatalf("quotePostgresIdentifier() error = %v", err)
	}
	if quoted != `"public"."data"` {
		t.Fatalf("quoted identifier = %q", quoted)
	}
	if _, err := quotePostgresIdentifier("data; DROP TABLE data"); err == nil {
		t.Fatal("unsafe identifier was accepted")
	}
}
