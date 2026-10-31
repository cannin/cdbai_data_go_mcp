# Repository Guidelines

## Commands

- Download and reconcile dependencies: `go mod tidy`
- Format: `gofmt -w .`
- Test: `go test ./...`
- Static checks: `go vet ./...`
- Build current platform: `make current`
- Full verification and releases: `make`

## Architecture

- `cmd/cdbai_data_go_mcp/main.go` wires configuration, BigQuery, and the MCP transports.
- `internal/config` owns environment parsing and identifier validation.
- `internal/data` owns typed inputs, parameterized SQL construction, query execution, and result shaping.
- The server exposes typed retrieval only. Do not add arbitrary SQL or model-generated code execution.

## Editing Constraints

- Keep the configured table fixed server-side.
- Send every client-provided value through BigQuery named parameters.
- Preserve row, billed-byte, and timeout safeguards.
- Keep stdout reserved for MCP protocol traffic; diagnostics go to stderr.
- Load `.env` without overriding variables already exported by the MCP client; `--env FILE` selects another file.

## Testing Requirements

- Run `gofmt -w .`, `go test ./...`, and `go vet ./...` after changes.
- Add tests for input validation, query parameterization, row caps, and truncation behavior.
- BigQuery unit tests must use fakes and must not require live credentials.

## Repository Pitfalls

- Release only Linux AMD64 and macOS ARM64 binaries.
- Streamable HTTP is the default transport at `127.0.0.1:8080/mcp`; stdio remains available with `--transport stdio`.
- MCP stdio breaks if application logs are written to stdout.
- A SQL `LIMIT` does not replace `MaxBytesBilled`; preserve both.
- Table identifiers cannot be query parameters, so they must remain environment-owned and strictly validated.
