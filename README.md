# cdbai_data_go_mcp

Read-only MCP server for retrieving CellMinerCDB rows from BigQuery without LLM calls or generated code.

Version: `0.2.1`

## Quick Start

```sh
cp .env.example .env
# Edit .env with your BigQuery settings.
make current
./dist/cdbai_data_go_mcp
```

The command automatically reads `.env` from its working directory. Exported environment variables take precedence. By default, it serves MCP over Streamable HTTP at `http://127.0.0.1:8080/mcp`; `http://127.0.0.1:8080/healthz` is available for health checks.

Use a different dotenv path with:

```sh
./dist/cdbai_data_go_mcp --env /path/to/cdbai.env
```

## Installation

Requirements:

- Go 1.26+
- BigQuery credentials available through Google Application Default Credentials
- Read access to the configured table

Build the current-platform binary:

```sh
make current
```

Run tests and build every supported release target:

```sh
make
```

Tagged versions matching `vVERSION` publish Linux AMD64 and macOS ARM64 binaries through [GitHub Releases](https://github.com/cannin/cdbai_data_go_mcp/releases).

## Basic Usage

Run the binary on the same host as Caddy and select the production MCP path:

```sh
./dist/cdbai_data_go_mcp \
    --listen 127.0.0.1:8080 \
    --http-path /cdbai_data/mcp
```

Proxy the public MCP endpoint directly to the binary; Supergateway is not needed. Override the upstream `Host` header so the MCP SDK's localhost DNS-rebinding protection accepts requests forwarded by Caddy:

```caddyfile
cdbai.sciluna.com {
    reverse_proxy /cdbai_data/mcp 127.0.0.1:8080 {
        header_up Host 127.0.0.1:8080
    }
}
```

Then configure Codex with the HTTPS URL:

```sh
codex mcp add cdbai-data --url https://cdbai.sciluna.com/cdbai_data/mcp
```

The equivalent `config.toml` entry is:

```toml
[mcp_servers.cdbai-data]
url = "https://cdbai.sciluna.com/cdbai_data/mcp"
```

For local clients that still launch subprocesses, stdio remains available:

```json
{
  "mcpServers": {
    "cdbai-data": {
      "command": "/absolute/path/to/cdbai_data_go_mcp/dist/cdbai_data_go_mcp",
      "args": [
        "--transport", "stdio",
        "--env", "/absolute/path/to/cdbai_data_go_mcp/.env"
      ]
    }
  }
}
```

The server exposes one tool:

- `retrieve_data`: Retrieves long-format rows with optional lists of `data_types`, `datasets`, `obs_names`, `var_names`, and `tissues`, plus a bounded `limit`.

Filters are case-sensitive exact matches. The server trims surrounding whitespace but does not apply `LOWER()` or change identifier case. Use canonical values exactly as described by MCP discovery metadata. Different filter fields are combined with `AND`; values within one field are combined with `OR` semantics.

The MCP discovery metadata tells clients that `retrieve_data` is not synonym-aware and that aliases must be converted before the call. Cell lines require canonical Cellosaurus accessions, human genes require approved HGNC symbols, and compounds require numeric PubChem CIDs where applicable. For example, `MCF7` or `MCF-7` becomes `CVCL_0031`, `p53` becomes `TP53`, and `HER2` becomes `ERBB2`. Ambiguous identifiers must not be guessed. A zero-row response is inconclusive because it can mean either an invalid identifier or no matching observations.

Successful responses include `query_metrics` with the opaque BigQuery job ID, cache status, actual bytes processed and billed, total slot milliseconds, and an estimated on-demand query cost. The estimate uses the included public USD-per-TiB rate and does not apply free-tier credits, capacity commitments, negotiated pricing, or other billing adjustments. It does not expose SQL text, table identifiers, project identifiers, credentials, or billing-account details.

### Command-line options

| Option | Default | Purpose |
|---|---|---|
| `-t, --transport` | `http` | MCP transport: `http` or `stdio` |
| `-l, --listen` | `127.0.0.1:8080` | Streamable HTTP listen address |
| `--http-path` | `/mcp` | Streamable HTTP endpoint path |
| `-e, --env` | `.env` | dotenv configuration file |

## Configuration

| Variable | Required | Default | Purpose |
|---|---:|---:|---|
| `GOOGLE_PROJECT_ID` | yes | - | Google Cloud project and table project |
| `GOOGLE_DATASET` | yes | - | BigQuery dataset |
| `GOOGLE_TABLE` | yes | - | BigQuery table |
| `GOOGLE_APPLICATION_CREDENTIALS` | no | ADC | Optional service-account JSON path |
| `CDBAI_MAX_ROWS` | no | `1000` | Hard output-row limit per call |
| `CDBAI_MAX_BYTES_BILLED` | no | `1000000000` | BigQuery billed-byte ceiling per query |
| `CDBAI_QUERY_TIMEOUT` | no | `30s` | Per-query Go duration |

## Safety

The MCP does not accept SQL. It interpolates only validated server-side table identifiers and sends all user filters as named BigQuery parameters. Each query is read-only, time-bounded, row-bounded, and protected by a maximum billed-byte setting. The Google identity should also have a read-only BigQuery role.
