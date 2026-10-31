// Command cdbai_data_go_mcp serves read-only CellMinerCDB data over MCP.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/cannin/cdbai_data_go_mcp/internal/config"
	"github.com/cannin/cdbai_data_go_mcp/internal/data"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const version = "0.0.1"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

// run configures the BigQuery client and serves the MCP protocol over stdio.
//
// Args:
//   - parent: Parent context used to stop the server.
//   - args: Command-line arguments excluding the program name.
//
// Returns:
//   - error: Configuration, client, or server error.
func run(parent context.Context, args []string) error {
	ctx, stop := signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	envFile, err := parseArgs(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(envFile)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	client, err := bigquery.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return fmt.Errorf("create BigQuery client: %w", err)
	}
	defer client.Close()

	executor := data.NewBigQueryExecutor(client, cfg.MaxBytesBilled)
	service := data.NewService(executor, cfg.TableFQN(), cfg.MaxRows)
	server := newServer(service, cfg.QueryTimeout)

	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
		return fmt.Errorf("run MCP server: %w", err)
	}
	return nil
}

// parseArgs returns the dotenv path selected on the command line.
func parseArgs(args []string) (string, error) {
	flags := flag.NewFlagSet("cdbai_data_go_mcp", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	envFile := ".env"
	flags.StringVar(&envFile, "env", envFile, "dotenv configuration file")
	flags.StringVar(&envFile, "e", envFile, "dotenv configuration file")
	if err := flags.Parse(args); err != nil {
		return "", fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return "", fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}
	return envFile, nil
}

// newServer registers the typed, read-only retrieval tool.
//
// Args:
//   - service: Data retrieval service used by the tool handler.
//   - queryTimeout: Maximum duration of one tool query.
//
// Returns:
//   - *mcp.Server: Configured MCP server.
func newServer(service *data.Service, queryTimeout time.Duration) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "cdbai-data", Version: version},
		nil,
	)
	closedWorld := false
	destructive := false
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:  "retrieve_data",
			Title: "Retrieve CellMinerCDB data",
			Description: "Retrieve rows from the configured CellMinerCDB BigQuery table. " +
				"Filters are optional exact-match lists and are combined with AND. " +
				"Known data_type values include act, cop, mut, met, mth, bmt, rrb, exp, " +
				"xai, pro, mir, mda, swa, xsq, cri, mtb, his, hs4, and sur. " +
				"The tool is read-only and never accepts SQL.",
			Annotations: &mcp.ToolAnnotations{
				DestructiveHint: &destructive,
				IdempotentHint:  true,
				OpenWorldHint:   &closedWorld,
				ReadOnlyHint:    true,
			},
		},
		func(
			callCtx context.Context,
			_ *mcp.CallToolRequest,
			input data.RetrieveInput,
		) (*mcp.CallToolResult, data.RetrieveOutput, error) {
			queryCtx, cancel := context.WithTimeout(callCtx, queryTimeout)
			defer cancel()

			output, retrieveErr := service.Retrieve(queryCtx, input)
			return nil, output, retrieveErr
		},
	)
	return server
}
