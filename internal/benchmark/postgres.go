package benchmark

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var postgresIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// PostgresRunner executes equivalent Cloud SQL PostgreSQL retrieval trials.
type PostgresRunner struct {
	connection *pgx.Conn
	table      string
	dataType   string
	dataset    string
}

// NewPostgresRunner validates the table identifier and creates a backend.
func NewPostgresRunner(
	connection *pgx.Conn,
	table string,
	dataType string,
	dataset string,
) (*PostgresRunner, error) {
	quotedTable, err := quotePostgresIdentifier(table)
	if err != nil {
		return nil, err
	}
	return &PostgresRunner{
		connection: connection,
		table:      quotedTable,
		dataType:   strings.ToLower(dataType),
		dataset:    strings.ToLower(dataset),
	}, nil
}

// Name returns the backend identifier used in reports.
func (r *PostgresRunner) Name() string {
	return "postgres"
}

// Run executes one PostgreSQL retrieval and consumes the result rows.
func (r *PostgresRunner) Run(ctx context.Context, limit int) (Result, error) {
	query := fmt.Sprintf(
		"SELECT data_type, dataset, obs_name, var_name, value, tissue "+
			"FROM %s "+
			"WHERE value IS NOT NULL "+
			"AND LOWER(data_type) = $1 "+
			"AND LOWER(dataset) = $2 "+
			"LIMIT $3",
		r.table,
	)
	started := time.Now()
	rows, err := r.connection.Query(ctx, query, r.dataType, r.dataset, limit)
	if err != nil {
		return Result{}, fmt.Errorf("start query: %w", err)
	}
	defer rows.Close()

	rowCount := 0
	for rows.Next() {
		var dataType string
		var dataset string
		var obsName string
		var varName string
		var value float64
		var tissue *string
		if err := rows.Scan(&dataType, &dataset, &obsName, &varName, &value, &tissue); err != nil {
			return Result{}, fmt.Errorf("scan query row: %w", err)
		}
		rowCount++
	}
	if err := rows.Err(); err != nil {
		return Result{}, fmt.Errorf("iterate query: %w", err)
	}
	return Result{
		RequestedRows: limit,
		ReturnedRows:  rowCount,
		DurationMS:    float64(time.Since(started).Microseconds()) / 1_000,
	}, nil
}

func quotePostgresIdentifier(identifier string) (string, error) {
	parts := strings.Split(identifier, ".")
	if len(parts) < 1 || len(parts) > 2 {
		return "", fmt.Errorf("POSTGRES_TABLE must be table or schema.table")
	}
	for index, part := range parts {
		if !postgresIdentifierPattern.MatchString(part) {
			return "", fmt.Errorf("POSTGRES_TABLE contains unsupported characters")
		}
		parts[index] = `"` + part + `"`
	}
	return strings.Join(parts, "."), nil
}
