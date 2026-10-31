package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("GOOGLE_PROJECT_ID", "example-project")
	t.Setenv("GOOGLE_DATASET", "cdb")
	t.Setenv("GOOGLE_TABLE", "data")
	t.Setenv("CDBAI_MAX_ROWS", "")
	t.Setenv("CDBAI_MAX_BYTES_BILLED", "")
	t.Setenv("CDBAI_QUERY_TIMEOUT", "")

	cfg, err := Load(".env")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TableFQN() != "example-project.cdb.data" {
		t.Fatalf("TableFQN() = %q", cfg.TableFQN())
	}
	if cfg.MaxRows != defaultMaxRows {
		t.Fatalf("MaxRows = %d, want %d", cfg.MaxRows, defaultMaxRows)
	}
	if cfg.QueryTimeout != 30*time.Second {
		t.Fatalf("QueryTimeout = %s", cfg.QueryTimeout)
	}
}

func TestLoadRejectsUnsafeIdentifier(t *testing.T) {
	t.Setenv("GOOGLE_PROJECT_ID", "example-project")
	t.Setenv("GOOGLE_DATASET", "cdb` WHERE TRUE --")
	t.Setenv("GOOGLE_TABLE", "data")

	if _, err := Load(".env"); err == nil {
		t.Fatal("Load() error = nil, want invalid identifier error")
	}
}

func TestLoadRejectsInvalidLimit(t *testing.T) {
	t.Setenv("GOOGLE_PROJECT_ID", "example-project")
	t.Setenv("GOOGLE_DATASET", "cdb")
	t.Setenv("GOOGLE_TABLE", "data")
	t.Setenv("CDBAI_MAX_ROWS", "0")

	if _, err := Load(".env"); err == nil {
		t.Fatal("Load() error = nil, want positive integer error")
	}
}

func TestLoadReadsDotEnv(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	contents := []byte("GOOGLE_PROJECT_ID=dotenv-project\nGOOGLE_DATASET=dotenv_dataset\nGOOGLE_TABLE=dotenv_table\n")
	if err := os.WriteFile(envPath, contents, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	t.Setenv("GOOGLE_PROJECT_ID", "")
	t.Setenv("GOOGLE_DATASET", "")
	t.Setenv("GOOGLE_TABLE", "")
	os.Unsetenv("GOOGLE_PROJECT_ID")
	os.Unsetenv("GOOGLE_DATASET")
	os.Unsetenv("GOOGLE_TABLE")

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TableFQN() != "dotenv-project.dotenv_dataset.dotenv_table" {
		t.Fatalf("TableFQN() = %q", cfg.TableFQN())
	}
}

func TestLoadDotEnvDoesNotOverrideExportedValues(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), ".env")
	contents := []byte("GOOGLE_PROJECT_ID=dotenv-project\nGOOGLE_DATASET=dotenv_dataset\nGOOGLE_TABLE=dotenv_table\n")
	if err := os.WriteFile(envPath, contents, 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}
	t.Setenv("GOOGLE_PROJECT_ID", "exported-project")
	t.Setenv("GOOGLE_DATASET", "exported_dataset")
	t.Setenv("GOOGLE_TABLE", "exported_table")

	cfg, err := Load(envPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.TableFQN() != "exported-project.exported_dataset.exported_table" {
		t.Fatalf("TableFQN() = %q", cfg.TableFQN())
	}
}

func TestLoadRejectsMissingExplicitEnvFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.env")
	if _, err := Load(missingPath); err == nil {
		t.Fatal("Load() error = nil, want missing file error")
	}
}
