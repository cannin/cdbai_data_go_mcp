package main

import (
	"context"
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

func (protocolTestExecutor) Execute(_ context.Context, _ data.QuerySpec) ([]data.Row, error) {
	return []data.Row{{
		DataType: "exp",
		Dataset:  "ccle",
		ObsName:  "CVCL_0002",
		VarName:  "TP53",
		Value:    1.5,
	}}, nil
}

func TestParseArgs(t *testing.T) {
	envFile, err := parseArgs([]string{"--env", "/tmp/cdbai.env"})
	if err != nil {
		t.Fatalf("parseArgs() error = %v", err)
	}
	if envFile != "/tmp/cdbai.env" {
		t.Fatalf("parseArgs() = %q", envFile)
	}
}

func TestMCPToolDiscoveryAndCall(t *testing.T) {
	ctx := context.Background()
	service := data.NewService(protocolTestExecutor{}, "project.dataset.table", 1_000)
	server := newServer(service, time.Second)
	client := mcp.NewClient(
		&mcp.Implementation{Name: "test-client", Version: "0.0.1"},
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
	command := exec.CommandContext(ctx, binaryPath, "--env", envPath)
	command.Dir = repositoryRoot

	client := mcp.NewClient(
		&mcp.Implementation{Name: "live-test-client", Version: "0.0.1"},
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
}
