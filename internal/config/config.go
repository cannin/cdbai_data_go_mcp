// Package config loads and validates server configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultMaxRows        = 1_000
	defaultMaxBytesBilled = 1_000_000_000
	defaultQueryTimeout   = 30 * time.Second
)

var identifierPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Config contains validated BigQuery and safety settings.
type Config struct {
	ProjectID      string
	Dataset        string
	Table          string
	MaxRows        int
	MaxBytesBilled int64
	QueryTimeout   time.Duration
}

// Load reads a dotenv file and then validates environment variables.
//
// Args:
//   - envFile: Dotenv path. A missing default .env file is allowed.
//
// Returns:
//   - Config: Validated server configuration.
//   - error: Missing or invalid configuration.
func Load(envFile string) (Config, error) {
	if err := loadDotEnv(envFile); err != nil {
		return Config{}, err
	}

	projectID, err := requiredEnv("GOOGLE_PROJECT_ID")
	if err != nil {
		return Config{}, err
	}
	dataset, err := requiredEnv("GOOGLE_DATASET")
	if err != nil {
		return Config{}, err
	}
	table, err := requiredEnv("GOOGLE_TABLE")
	if err != nil {
		return Config{}, err
	}

	for name, value := range map[string]string{
		"GOOGLE_PROJECT_ID": projectID,
		"GOOGLE_DATASET":    dataset,
		"GOOGLE_TABLE":      table,
	} {
		if !identifierPattern.MatchString(value) {
			return Config{}, fmt.Errorf("%s contains unsupported characters", name)
		}
	}

	maxRows, err := positiveIntEnv("CDBAI_MAX_ROWS", defaultMaxRows)
	if err != nil {
		return Config{}, err
	}
	maxBytes, err := positiveInt64Env("CDBAI_MAX_BYTES_BILLED", defaultMaxBytesBilled)
	if err != nil {
		return Config{}, err
	}
	queryTimeout, err := durationEnv("CDBAI_QUERY_TIMEOUT", defaultQueryTimeout)
	if err != nil {
		return Config{}, err
	}

	return Config{
		ProjectID:      projectID,
		Dataset:        dataset,
		Table:          table,
		MaxRows:        maxRows,
		MaxBytesBilled: maxBytes,
		QueryTimeout:   queryTimeout,
	}, nil
}

// loadDotEnv loads local configuration without replacing exported variables.
func loadDotEnv(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("environment file path cannot be empty")
	}
	if err := godotenv.Load(path); err != nil {
		if errors.Is(err, os.ErrNotExist) && path == ".env" {
			return nil
		}
		return fmt.Errorf("load environment file %q: %w", path, err)
	}
	return nil
}

// TableFQN returns the validated fully qualified BigQuery table name.
//
// Returns:
//   - string: project.dataset.table identifier.
func (c Config) TableFQN() string {
	return fmt.Sprintf("%s.%s.%s", c.ProjectID, c.Dataset, c.Table)
}

func requiredEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func positiveIntEnv(name string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func positiveInt64Env(name string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return value, nil
}

func durationEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive Go duration such as 30s", name)
	}
	return value, nil
}
