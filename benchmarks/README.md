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
