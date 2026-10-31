// Command db_benchmark compares direct BigQuery and PostgreSQL retrieval speed.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/bigquery"
	benchmarkpkg "github.com/cannin/cdbai_data_go_mcp/internal/benchmark"
	"github.com/cannin/cdbai_data_go_mcp/internal/config"
	"github.com/cannin/cdbai_data_go_mcp/internal/data"
	"github.com/jackc/pgx/v5"
)

const defaultLimits = "1,2,5,10,20,50,100,200,500,1000"

type options struct {
	backend   string
	envFile   string
	limits    []int
	outputDir string
	dataType  string
	dataset   string
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	options, err := parseOptions(args)
	if err != nil {
		return err
	}
	if err := config.LoadEnvFile(options.envFile); err != nil {
		return err
	}

	switch options.backend {
	case "bigquery":
		return runBigQuery(ctx, options)
	case "postgres":
		return runPostgres(ctx, options)
	case "both":
		if err := runBigQuery(ctx, options); err != nil {
			return err
		}
		return runPostgres(ctx, options)
	default:
		return fmt.Errorf("backend must be bigquery, postgres, or both")
	}
}

func runBigQuery(ctx context.Context, options options) error {
	cfg, err := config.Load(options.envFile)
	if err != nil {
		return err
	}
	client, err := bigquery.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("create BigQuery client: %w", err)
	}
	defer client.Close()

	runner := benchmarkpkg.NewBigQueryRunner(
		client,
		cfg.TableFQN(),
		cfg.MaxBytesBilled,
		data.RetrieveInput{DataTypes: []string{options.dataType}, Datasets: []string{options.dataset}},
	)
	return executeAndWrite(ctx, runner, options)
}

func runPostgres(ctx context.Context, options options) error {
	dsn := strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
	if dsn == "" {
		return fmt.Errorf("POSTGRES_DSN is required for the postgres backend")
	}
	table := strings.TrimSpace(os.Getenv("POSTGRES_TABLE"))
	if table == "" {
		table = "data"
	}
	connection, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer connection.Close(ctx)
	runner, err := benchmarkpkg.NewPostgresRunner(
		connection,
		table,
		options.dataType,
		options.dataset,
	)
	if err != nil {
		return err
	}
	return executeAndWrite(ctx, runner, options)
}

func executeAndWrite(ctx context.Context, runner benchmarkpkg.Runner, options options) error {
	benchmarkCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()
	report, err := benchmarkpkg.Execute(
		benchmarkCtx,
		runner,
		options.limits,
		options.dataType,
		options.dataset,
	)
	if err != nil {
		return err
	}
	jsonPath, csvPath, err := benchmarkpkg.WriteReport(options.outputDir, report)
	if err != nil {
		return err
	}
	log.Printf(
		"%s: %d runs, mean %.3f ms, median %.3f ms, p95 %.3f ms",
		report.Backend,
		report.Summary.Runs,
		report.Summary.MeanDurationMS,
		report.Summary.MedianDurationMS,
		report.Summary.P95DurationMS,
	)
	log.Printf("reports: %s and %s", jsonPath, csvPath)
	return nil
}

func parseOptions(args []string) (options, error) {
	flags := flag.NewFlagSet("db_benchmark", flag.ContinueOnError)
	backend := flags.String("backend", "bigquery", "bigquery, postgres, or both")
	envFile := flags.String("env", ".env", "dotenv configuration file")
	limitsText := flags.String("limits", defaultLimits, "comma-separated requested row amounts")
	outputDir := flags.String("output-dir", "benchmarks/results", "result artifact directory")
	dataType := flags.String("data-type", "cop", "data_type exact-match filter")
	dataset := flags.String("dataset", "ctrp", "dataset exact-match filter")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	limits, err := parseLimits(*limitsText)
	if err != nil {
		return options{}, err
	}
	return options{
		backend:   strings.ToLower(strings.TrimSpace(*backend)),
		envFile:   *envFile,
		limits:    limits,
		outputDir: *outputDir,
		dataType:  strings.TrimSpace(*dataType),
		dataset:   strings.TrimSpace(*dataset),
	}, nil
}

func parseLimits(text string) ([]int, error) {
	parts := strings.Split(text, ",")
	limits := make([]int, 0, len(parts))
	for _, part := range parts {
		limit, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || limit <= 0 {
			return nil, fmt.Errorf("limits must contain positive integers")
		}
		limits = append(limits, limit)
	}
	if len(limits) == 0 {
		return nil, fmt.Errorf("at least one limit is required")
	}
	return limits, nil
}
