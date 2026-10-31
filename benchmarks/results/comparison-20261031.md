# BigQuery versus PostgreSQL retrieval benchmark

Both backends queried the same six-column dataset with the filters
`data_type=cop` and `dataset=ctrp`. Each backend retrieved limits of
`1, 2, 5, 10, 20, 50, 100, 200, 500, 1000` rows.

| Metric | BigQuery | PostgreSQL | PostgreSQL speedup |
|---|---:|---:|---:|
| Mean | 1,294.344 ms | 5.342 ms | 242.3x |
| Median | 1,261.999 ms | 1.336 ms | 945.0x |
| P95 | 1,798.166 ms | 37.057 ms | 48.5x |

BigQuery scanned about 17.409 GB for every query regardless of the row limit,
and billed 174.095 GB across all ten runs. PostgreSQL used an index on
`LOWER(data_type), LOWER(dataset)` and does not charge by bytes scanned.

These measurements favor the repeated, selective retrieval pattern used by
the MCP. They are not a general comparison of analytical workloads: BigQuery
remains better suited to large aggregations and scans, while PostgreSQL offers
substantially lower latency and predictable instance-based cost for these
indexed lookups.
