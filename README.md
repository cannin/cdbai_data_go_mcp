# cdbai_data_go_mcp

Read-only MCP server for retrieving CellMinerCDB rows from BigQuery without LLM calls or generated code.

Version: `0.0.1`

## Quick Start

```sh
cp .env.example .env
# Edit .env with your BigQuery settings.
make current
./dist/cdbai_data_go_mcp
```

The command automatically reads `.env` from its working directory. Exported environment variables take precedence. The server uses MCP over stdin/stdout, so it normally runs as a child process of an MCP client rather than directly in a terminal.

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

Configure an MCP client to launch the binary. The generic configuration shape is:

```json
{
  "mcpServers": {
    "cdbai-data": {
      "command": "/absolute/path/to/cdbai_data_go_mcp/dist/cdbai_data_go_mcp",
      "args": ["--env", "/absolute/path/to/cdbai_data_go_mcp/.env"]
    }
  }
}
```

The server exposes one tool:

- `retrieve_data`: Retrieves long-format rows with optional lists of `data_types`, `datasets`, `obs_names`, `var_names`, and `tissues`, plus a bounded `limit`.

Filters are case-insensitive exact matches. Different filter fields are combined with `AND`; values within one field are combined with `OR` semantics.

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
