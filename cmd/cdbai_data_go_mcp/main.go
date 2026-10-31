// Command cdbai_data_go_mcp serves read-only CellMinerCDB data over MCP.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/cannin/cdbai_data_go_mcp/internal/config"
	"github.com/cannin/cdbai_data_go_mcp/internal/data"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	version                 = "0.1.1"
	defaultListenAddress    = "127.0.0.1:8080"
	defaultMCPPath          = "/mcp"
	gracefulShutdownTimeout = 5 * time.Second
	serverInstructions      = `This server retrieves cancer pharmacogenomics data using exact-match biological identifiers. Resolve aliases to canonical identifiers before retrieval: use Cellosaurus accessions for cell lines, approved HGNC symbols for human genes, and numeric PubChem CIDs for compounds where applicable. For example, MCF7 or MCF-7 maps to CVCL_0031, p53 maps to TP53, and HER2 maps to ERBB2. Never invent an identifier; ask for clarification when a name is ambiguous. Treat a zero-row result as inconclusive because it may mean that an identifier is invalid or that no observations match the filters. Always inspect count, limit, and truncated before interpreting a response.

CellMinerCDB data is stored in long format. Each row contains data_type, dataset, obs_name, var_name, value, and tissue. The value is the numeric measurement for one obs_name and var_name pair.

All filters are case-sensitive exact matches. The server trims surrounding whitespace but does not apply LOWER(), uppercase values, resolve aliases, or otherwise change identifier case. Supply every value in its stored canonical form: data_type uses the lowercase codes below; dataset uses the exact spelling below, including internal capitals; obs_name uses an uppercase CVCL_ accession; HGNC var_name values use the approved uppercase gene symbol; PubChem var_name values contain digits only; other var_name modalities use their canonical database spelling; and tissue uses the exact spelling below.

Interpret data_type as follows:
- act: drug activity; var_name is a PubChem ID
- cop: DNA copy number; var_name is an HGNC gene symbol
- mut: DNA mutation; var_name is an HGNC gene symbol
- met: DNA methylation 450K; var_name is an HGNC gene symbol
- mth: DNA methylation 850K; var_name is an HGNC gene symbol
- bmt: body DNA methylation 850K; var_name is an HGNC gene symbol
- rrb: DNA methylation RRBS; var_name is an HGNC gene symbol
- exp: microarray RNA expression using z-score; var_name is an HGNC gene symbol
- xai: microarray RNA expression using Avg.log2; var_name is an HGNC gene symbol
- pro: RPLA protein; var_name is a protein or HGNC identifier
- mir: microRNA; var_name is a microRNA ID
- mda: miscellaneous phenotypic data; var_name is a miscellaneous phenotypic name
- swa: SWATH-MS protein; var_name is a protein or HGNC identifier
- xsq: RNA-seq expression using log2(FPKM + 1); var_name is an HGNC gene symbol
- cri: CRISPR; var_name is an HGNC gene symbol
- mtb: metabolomics; var_name is a metabolite
- his: histone H3K27ac; var_name is an HGNC gene symbol
- hs4: histone H3K4me3; var_name is an HGNC gene symbol
- sur: surface receptor protein; var_name is a protein or HGNC identifier

Interpret dataset as follows:
- acc: adrenocortical carcinoma
- achilles: Achilles CRISPR
- almanac: NCI ALMANAC dataset
- ccle: Broad Cancer Cell Line Encyclopedia
- ctrp: Broad Cancer Therapeutics Response Portal using CellTitreGlo
- gdsc1: drug response using resazurin or Syto60
- gdsc2: drug response using CellTitreGlo
- gdscDec15: GDSC December 2015 dataset
- mdaMills: MD Anderson proteomics
- nci60: NCI DTP drug response using sulforhodamine B and multigenomics
- nci60Hts384: NCI DTP drug response using CellTitreGlo
- nciSarcoma: NCI sarcoma dataset
- nciSclc: NCI small cell lung cancer drug response and multigenomics
- prism: Broad PRISM drug response using CellTitreGlo
- tncats: NIH NCATS drug response using CellTitreGlo
- uniSarcoma: unified sarcoma

obs_name is a case-sensitive Cellosaurus cell-line identifier and always starts with uppercase CVCL_. var_name depends on data_type: approved HGNC symbols use their canonical uppercase form, PubChem IDs are numeric only, and other identifiers retain the canonical case defined by their source database.

tissue is one of: Adrenal_Gland, Biliary_Tract, Bladder_Urinary_Tract, Blood, Bone, Bowel, Brain_CNS, Breast, Cervix, Esophagus_Stomach, Eye, Head_Neck, Kidney, Liver, Lung, Lymph, Mesothelium, Other, Ovary, Pancreas, Peripheral_Nervous_System, Prostate, Skin, Soft_Tissue, Thyroid, Uterus, or Vulva.

For example, data_type cop, dataset ctrp, obs_name CVCL_0002, var_name DDX25, value 0.042600, and tissue Blood represents a DNA copy-number measurement for DDX25 in that blood-derived cell line.`
	retrieveDataDescription = `Retrieve long-format CellMinerCDB rows from the configured BigQuery table. Each row contains data_type, dataset, obs_name, var_name, value, and tissue; use data_type to interpret var_name and value according to the server instructions. Optional filters are case-sensitive exact matches. The server trims surrounding whitespace but does not apply LOWER(), change identifier case, or resolve aliases. Values within one filter use OR semantics, while different filters are combined with AND. IMPORTANT: before calling the tool, resolve cell-line names to uppercase Cellosaurus accessions, human gene aliases to approved uppercase HGNC symbols, and compound names to numeric PubChem CIDs where applicable. Use dataset and tissue values exactly as listed in the server instructions, including internal capitals and underscores. Examples: MCF7 or MCF-7 -> CVCL_0031; p53 -> TP53; HER2 -> ERBB2. Never invent an identifier; ask for clarification when a name is ambiguous. Zero rows do not prove biological absence: they may indicate an invalid identifier, incorrectly cased input, or no observations for the selected filters. Inspect count, limit, and truncated before interpreting results. Known data_type values include act, cop, mut, met, mth, bmt, rrb, exp, xai, pro, mir, mda, swa, xsq, cri, mtb, his, hs4, and sur. This tool is read-only, row-limited, and never accepts SQL.`
)

type commandOptions struct {
	envFile       string
	transport     string
	listenAddress string
	mcpPath       string
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

// run configures the BigQuery client and serves the MCP protocol.
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

	options, err := parseArgs(args)
	if err != nil {
		return err
	}
	cfg, err := config.Load(options.envFile)
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

	if options.transport == "stdio" {
		if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil {
			return fmt.Errorf("run MCP server over stdio: %w", err)
		}
		return nil
	}

	return serveHTTP(ctx, server, options.listenAddress, options.mcpPath)
}

// parseArgs returns the server options selected on the command line.
func parseArgs(args []string) (commandOptions, error) {
	flags := flag.NewFlagSet("cdbai_data_go_mcp", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	options := commandOptions{
		envFile:       ".env",
		transport:     "http",
		listenAddress: defaultListenAddress,
		mcpPath:       defaultMCPPath,
	}
	flags.StringVar(&options.envFile, "env", options.envFile, "dotenv configuration file")
	flags.StringVar(&options.envFile, "e", options.envFile, "dotenv configuration file")
	flags.StringVar(&options.transport, "transport", options.transport, "MCP transport: http or stdio")
	flags.StringVar(&options.transport, "t", options.transport, "MCP transport: http or stdio")
	flags.StringVar(&options.listenAddress, "listen", options.listenAddress, "HTTP listen address")
	flags.StringVar(&options.listenAddress, "l", options.listenAddress, "HTTP listen address")
	flags.StringVar(&options.mcpPath, "http-path", options.mcpPath, "Streamable HTTP endpoint path")
	if err := flags.Parse(args); err != nil {
		return commandOptions{}, fmt.Errorf("parse arguments: %w", err)
	}
	if flags.NArg() != 0 {
		return commandOptions{}, fmt.Errorf("unexpected positional arguments: %v", flags.Args())
	}

	options.transport = strings.ToLower(strings.TrimSpace(options.transport))
	if options.transport != "http" && options.transport != "stdio" {
		return commandOptions{}, fmt.Errorf(
			"unsupported transport %q: use http or stdio",
			options.transport,
		)
	}
	if options.listenAddress == "" {
		return commandOptions{}, errors.New("listen address cannot be empty")
	}
	if !strings.HasPrefix(options.mcpPath, "/") || strings.ContainsAny(options.mcpPath, "?#") {
		return commandOptions{}, fmt.Errorf(
			"invalid HTTP path %q: use an absolute path without a query or fragment",
			options.mcpPath,
		)
	}
	if options.mcpPath == "/healthz" {
		return commandOptions{}, errors.New("HTTP path /healthz is reserved for health checks")
	}

	return options, nil
}

// serveHTTP serves MCP over Streamable HTTP until the context is canceled.
//
// Args:
//   - ctx: Context used to stop the HTTP server.
//   - server: Configured MCP server.
//   - listenAddress: TCP address on which the server listens.
//   - mcpPath: URL path that accepts Streamable HTTP requests.
//
// Returns:
//   - error: Listener, server, or graceful shutdown error.
func serveHTTP(
	ctx context.Context,
	server *mcp.Server,
	listenAddress string,
	mcpPath string,
) error {
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", listenAddress, err)
	}

	httpServer := &http.Server{
		Addr:              listenAddress,
		Handler:           newHTTPHandler(server, mcpPath),
		ReadHeaderTimeout: 5 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- httpServer.Serve(listener)
	}()

	log.Printf("serving MCP Streamable HTTP on http://%s%s", listener.Addr(), mcpPath)
	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve MCP Streamable HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownTimeout)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down MCP Streamable HTTP server: %w", err)
		}
		return nil
	}
}

// newHTTPHandler returns the Streamable HTTP and health-check routes.
//
// Args:
//   - server: Configured MCP server.
//   - mcpPath: URL path that accepts Streamable HTTP requests.
//
// Returns:
//   - http.Handler: Handler for the MCP and health-check endpoints.
func newHTTPHandler(server *mcp.Server, mcpPath string) http.Handler {
	streamableHandler := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{
			Stateless:                    true,
			PropagateRequestCancellation: true,
		},
	)
	mux := http.NewServeMux()
	mux.Handle(mcpPath, streamableHandler)
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.WriteHeader(http.StatusOK)
		_, _ = response.Write([]byte("ok\n"))
	})
	return mux
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
		&mcp.ServerOptions{Instructions: serverInstructions},
	)
	closedWorld := false
	destructive := false
	mcp.AddTool(
		server,
		&mcp.Tool{
			Name:        "retrieve_data",
			Title:       "Retrieve CellMinerCDB data",
			Description: retrieveDataDescription,
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
