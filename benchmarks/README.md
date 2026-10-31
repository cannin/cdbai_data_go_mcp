# Database retrieval benchmark

This benchmark compares direct BigQuery and PostgreSQL retrieval of the same long-format data. It records timing and row counts only; returned scientific data is consumed but not stored.

The default ten requested row counts are `1, 2, 5, 10, 20, 50, 100, 200, 500, 1000`. BigQuery query caching is disabled so each trial measures an actual query and reports bytes processed and billed.

Run BigQuery:

```sh
go run ./cmd/db_benchmark --backend bigquery --env .env
```

For Cloud SQL PostgreSQL, load the same six-column table and add these private variables to `.env`:

```dotenv
POSTGRES_DSN=postgres://user:password@host:5432/database
POSTGRES_TABLE=public.data
```

Then run:

```sh
go run ./cmd/db_benchmark --backend postgres --env .env
```

Use `--backend both` when both databases are configured. Reports are written as JSON and CSV under `benchmarks/results/`.

## Recorded comparison

The 2026-10-31 run found a 1,294.344 ms BigQuery mean and a 5.342 ms PostgreSQL
mean for the same ten selective retrievals. See
[`results/comparison-20261031.md`](results/comparison-20261031.md) for the full
comparison and its scope.

The Cloud SQL import helpers are under `cloud_sql/`. `schema.sql` creates the
six-column table, `index.sql` adds the retrieval index, `compose_export.sh`
combines a sharded BigQuery export, and `run_vm_benchmark.sh` runs the benchmark
from a private Compute Engine VM.
