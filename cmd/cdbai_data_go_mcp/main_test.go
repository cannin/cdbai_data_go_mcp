package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cannin/cdbai_data_go_mcp/internal/data"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type protocolTestExecutor struct{}

func (protocolTestExecutor) Execute(_ context.Context, _ data.QuerySpec) (data.ExecutionResult, error) {
	return data.ExecutionResult{
		Rows: []data.Row{{
			DataType: "exp",
			Dataset:  "ccle",
			ObsName:  "CVCL_0002",
			VarName:  "TP53",
			Value:    1.5,
		}},
		Metrics: data.QueryMetrics{
			JobID:                    "job-123",
			TotalBytesProcessed:      100,
			TotalBytesBilled:         200,
			TotalSlotMilliseconds:    300,
			EstimatedOnDemandCostUSD: 0.001,
			OnDemandPricePerTiBUSD:   6.25,
		},
	}, nil
}

func TestParseArgs(t *testing.T) {
	options, err := parseArgs([]string{
		"--env", "/tmp/cdbai.env",
		"--listen", "localhost:9000",
		"--http-path", "/cdbai_data/mcp",
	})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if options.envFile != "/tmp/cdbai.env" {
		t.Fatalf("parseArgs() envFile = %q", options.envFile)
	}
	if options.transport != "http" {
		t.Fatalf("parseArgs() transport = %q", options.transport)
	}
	if options.listenAddress != "localhost:9000" {
		t.Fatalf("parseArgs() listenAddress = %q", options.listenAddress)
	}
	if options.mcpPath != "/cdbai_data/mcp" {
		t.Fatalf("parseArgs() mcpPath = %q", options.mcpPath)
	}
}

func TestParseArgsRejectsInvalidTransport(t *testing.T) {
	_, err := parseArgs([]string{"--transport", "websocket"})
	if err == nil || !strings.Contains(err.Error(), "unsupported transport") {
		t.Fatalf("parseArgs() error = %v, want unsupported transport", err)
	}
}

func TestMCPToolDiscoveryAndCall(t *testing.T) {
	ctx := context.Background()
	service := data.NewService(protocolTestExecutor{}, "project.dataset.table", 1_000)
	server := newServer(service, time.Second)
	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: version},
		nil,
	)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer clientSession.Close()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "retrieve_data" {
		t.Fatalf("ListTools() tools = %#v", tools.Tools)
	}
	if !tools.Tools[0].Annotations.ReadOnlyHint {
		t.Fatal("retrieve_data is not marked read-only")
	}
	initializeResult := clientSession.InitializeResult()
	if initializeResult == nil {
		t.Fatal("InitializeResult() is nil")
	}
	for _, expected := range []string{
		"Resolve aliases to canonical identifiers",
		"MCF7 or MCF-7 maps to CVCL_0031",
		"p53 maps to TP53",
		"zero-row result as inconclusive",
		"does not apply LOWER()",
		"mdaMills: MD Anderson proteomics",
		"nci60Hts384: NCI DTP drug response",
		"almanac: NCI ALMANAC dataset",
		"gdscDec15: GDSC December 2015 dataset",
		"nciSarcoma: NCI sarcoma dataset",
		"Adrenal_Gland",
		"Soft_Tissue",
		"Vulva",
	} {
		if !strings.Contains(initializeResult.Instructions, expected) {
			t.Errorf("server instructions do not contain %q: %q", expected, initializeResult.Instructions)
		}
	}

	tool := tools.Tools[0]
	for _, expected := range []string{
		"case-sensitive exact matches",
		"does not apply LOWER()",
		"query_metrics",
		"actual bytes processed and billed",
		"MCF7 or MCF-7 -> CVCL_0031",
		"p53 -> TP53",
		"Zero rows do not prove biological absence",
	} {
		if !strings.Contains(tool.Description, expected) {
			t.Errorf("tool description does not contain %q: %q", expected, tool.Description)
		}
	}

	schemaJSON, err := json.Marshal(tool.InputSchema)
	if err != nil {
		t.Fatalf("json.Marshal(InputSchema) error = %v", err)
	}
	schemaText := string(schemaJSON)
	for _, expected := range []string{
		"Case-sensitive canonical Cellosaurus accessions",
		"MCF7 or MCF-7 maps to CVCL_0031",
		"p53 maps to TP53 and HER2 maps to ERBB2",
		"gdscDec15, mdaMills, nci60, nci60Hts384, nciSarcoma, nciSclc",
		"Adrenal_Gland, Biliary_Tract",
		"Uterus, or Vulva",
		"Check truncated to determine whether the response is incomplete",
	} {
		if !strings.Contains(schemaText, expected) {
			t.Errorf("input schema does not contain %q: %s", expected, schemaText)
		}
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "retrieve_data",
		Arguments: map[string]any{"var_names": []string{"TP53"}, "limit": 10},
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() returned tool error: %#v", result.Content)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured content type = %T", result.StructuredContent)
	}
	if structured["count"] != float64(1) {
		t.Fatalf("structured count = %#v", structured["count"])
	}
	metrics, ok := structured["query_metrics"].(map[string]any)
	if !ok {
		t.Fatalf("query_metrics type = %T", structured["query_metrics"])
	}
	if metrics["job_id"] != "job-123" {
		t.Fatalf("query_metrics job_id = %#v", metrics["job_id"])
	}
	if metrics["total_bytes_billed"] != float64(200) {
		t.Fatalf("query_metrics total_bytes_billed = %#v", metrics["total_bytes_billed"])
	}
}

func TestStreamableHTTPToolDiscoveryAndCall(t *testing.T) {
	ctx := context.Background()
	service := data.NewService(protocolTestExecutor{}, "project.dataset.table", 1_000)
	server := newServer(service, time.Second)
	httpServer := httptest.NewServer(newHTTPHandler(server, "/cdbai_data/mcp"))
	defer httpServer.Close()

	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: version},
		nil,
	)
	clientSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint: httpServer.URL + "/cdbai_data/mcp",
	}, nil)
	if err != nil {
		t.Fatalf("connect over Streamable HTTP: %v", err)
	}
	defer clientSession.Close()

	tools, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools() over Streamable HTTP error = %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "retrieve_data" {
		t.Fatalf("ListTools() over Streamable HTTP tools = %#v", tools.Tools)
	}

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "retrieve_data",
		Arguments: map[string]any{"var_names": []string{"TP53"}, "limit": 10},
	})
	if err != nil {
		t.Fatalf("CallTool() over Streamable HTTP error = %v", err)
	}
	if result.IsError {
		t.Fatalf("CallTool() over Streamable HTTP returned tool error: %#v", result.Content)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok || structured["count"] != float64(1) {
		t.Fatalf("Streamable HTTP structured content = %#v", result.StructuredContent)
	}
}

func TestHTTPHealthCheck(t *testing.T) {
	service := data.NewService(protocolTestExecutor{}, "project.dataset.table", 1_000)
	httpServer := httptest.NewServer(newHTTPHandler(newServer(service, time.Second), "/mcp"))
	defer httpServer.Close()

	response, err := http.Get(httpServer.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz error = %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET /healthz status = %d", response.StatusCode)
	}
}

func TestLiveMCP(t *testing.T) {
	if os.Getenv("CDBAI_LIVE_TEST") != "1" {
		t.Skip("set CDBAI_LIVE_TEST=1 to run against configured BigQuery")
	}

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() could not locate the repository")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	binaryPath := filepath.Join(repositoryRoot, "dist", "cdbai_data_go_mcp")
	envPath := filepath.Join(repositoryRoot, ".env")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(
		ctx,
		binaryPath,
		"--transport", "stdio",
		"--env", envPath,
	)
	command.Dir = repositoryRoot

	client := mcp.NewClient(
		&mcp.Implementation{Name: "live-test-client", Version: version},
		nil,
	)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		t.Fatalf("connect to live MCP: %v", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("discover live MCP tools: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "retrieve_data" {
		t.Fatalf("live MCP tools = %#v", tools.Tools)
	}

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "retrieve_data",
		Arguments: map[string]any{
			"data_types": []string{"cop"},
			"datasets":   []string{"ctrp"},
			"obs_names":  []string{"CVCL_0002"},
			"var_names":  []string{"DDX25"},
			"limit":      1,
		},
	})
	if err != nil {
		t.Fatalf("call live retrieve_data: %v", err)
	}
	if result.IsError {
		messages := make([]string, 0, len(result.Content))
		for _, content := range result.Content {
			if text, isText := content.(*mcp.TextContent); isText {
				messages = append(messages, text.Text)
			}
		}
		t.Fatalf("live retrieve_data returned an error: %s", strings.Join(messages, "; "))
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("live structured content type = %T", result.StructuredContent)
	}
	if structured["count"] != float64(1) {
		t.Fatalf("live result count = %#v, want 1", structured["count"])
	}
	metrics, ok := structured["query_metrics"].(map[string]any)
	if !ok {
		t.Fatalf("live query_metrics type = %T", structured["query_metrics"])
	}
	jobID, ok := metrics["job_id"].(string)
	if !ok || jobID == "" {
		t.Fatalf("live query_metrics job_id = %#v", metrics["job_id"])
	}
	if bytesBilled, ok := metrics["total_bytes_billed"].(float64); !ok || bytesBilled < 0 {
		t.Fatalf("live query_metrics total_bytes_billed = %#v", metrics["total_bytes_billed"])
	}
	if estimatedCost, ok := metrics["estimated_on_demand_cost_usd"].(float64); !ok || estimatedCost < 0 {
		t.Fatalf(
			"live query_metrics estimated_on_demand_cost_usd = %#v",
			metrics["estimated_on_demand_cost_usd"],
		)
	}
	if price, ok := metrics["on_demand_price_per_tib_usd"].(float64); !ok || price != 6.25 {
		t.Fatalf(
			"live query_metrics on_demand_price_per_tib_usd = %#v",
			metrics["on_demand_price_per_tib_usd"],
		)
	}
}
