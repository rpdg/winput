package mcpadapter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadRequestContentLength(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	raw := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)

	req, err := readRequest(bufio.NewReader(bytes.NewBufferString(raw)))
	if err != nil {
		t.Fatalf("readRequest failed: %v", err)
	}
	if req.Method != "tools/list" {
		t.Fatalf("unexpected method: %s", req.Method)
	}
}

func TestMutatingToolsBlockedByDefault(t *testing.T) {
	server := NewServer(Config{})

	_, err := server.callTool(context.Background(), ToolCallParams{
		Name: "click",
		Arguments: map[string]any{
			"target_id": "window-1",
			"x":         1,
			"y":         1,
		},
	})
	if err == nil {
		t.Fatal("expected unsafe_operation error")
	}
	var coded *codedError
	if !strings.Contains(err.Error(), "mutating tools are disabled") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !errors.As(err, &coded) || coded.Code != "unsafe_operation" {
		t.Fatalf("expected unsafe_operation coded error, got %#v", err)
	}
}

func TestWriteResponseContentLength(t *testing.T) {
	var out bytes.Buffer
	resp := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Result:  map[string]any{"ok": true},
	}
	if err := writeResponse(&out, resp); err != nil {
		t.Fatalf("writeResponse failed: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "Content-Length: ") {
		t.Fatalf("missing content length header: %q", got)
	}
	if !strings.Contains(got, `{"jsonrpc":"2.0","id":1,"result":{"ok":true}}`) {
		t.Fatalf("missing payload: %q", got)
	}
}

func TestToolsListIncludesSchemas(t *testing.T) {
	server := NewServer(Config{})
	resp := server.handleRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "tools/list",
	})

	result, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %#v", resp.Result)
	}
	toolsRaw, ok := result["tools"].([]Tool)
	if !ok {
		t.Fatalf("unexpected tools type: %#v", result["tools"])
	}
	if len(toolsRaw) == 0 {
		t.Fatal("expected non-empty tool catalog")
	}
	if toolsRaw[0].OutputSchema == nil || toolsRaw[0].ErrorSchema == nil {
		t.Fatalf("expected outputSchema and errorSchema, got %#v", toolsRaw[0])
	}
}

func TestFindWindowOutputSchemaIncludesRuntimeMetadata(t *testing.T) {
	catalog := defaultCatalog()
	var findWindow Tool
	for _, tool := range catalog {
		if tool.Name == "find_window" {
			findWindow = tool
			break
		}
	}
	if findWindow.Name == "" {
		t.Fatal("find_window tool not found")
	}
	props, ok := findWindow.OutputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected output schema: %#v", findWindow.OutputSchema)
	}
	for _, field := range []string{"is_valid", "is_visible", "client_width", "client_height", "dpi_x", "dpi_y"} {
		if _, ok := props[field]; !ok {
			t.Fatalf("missing output schema field %q in %#v", field, props)
		}
	}
}

func TestMCPServerSmoke(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping MCP server smoke test in short mode")
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "run", "./cmd/mcp-server")
	cmd.Dir = repoRoot

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer func() {
		_ = stdin.Close()
		cancel()
		_ = cmd.Wait()
	}()

	outReader := bufio.NewReader(stdout)
	errBuf := new(bytes.Buffer)
	go func() {
		_, _ = io.Copy(errBuf, stderr)
	}()

	send := func(payload string) {
		t.Helper()
		frame := fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(payload), payload)
		if _, err := io.WriteString(stdin, frame); err != nil {
			t.Fatalf("write request: %v", err)
		}
	}

	readResp := func() JSONRPCResponse {
		t.Helper()
		resp, err := readFramedResponse(outReader)
		if err != nil {
			t.Fatalf("read response: %v\nstderr: %s", err, errBuf.String())
		}
		return *resp
	}

	send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	initResp := readResp()
	if initResp.Error != nil {
		t.Fatalf("initialize error: %#v", initResp.Error)
	}
	initResult, ok := initResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected initialize result: %#v", initResp.Result)
	}
	if initResult["protocolVersion"] != "2025-04-01" {
		t.Fatalf("unexpected protocolVersion: %#v", initResult["protocolVersion"])
	}

	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	listResp := readResp()
	if listResp.Error != nil {
		t.Fatalf("tools/list error: %#v", listResp.Error)
	}
	listResult, ok := listResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected tools/list result: %#v", listResp.Result)
	}
	toolsAny, ok := listResult["tools"].([]any)
	if !ok || len(toolsAny) == 0 {
		t.Fatalf("unexpected tools payload: %#v", listResult["tools"])
	}
	var foundBounds bool
	for _, item := range toolsAny {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if tool["name"] == "get_virtual_bounds" {
			foundBounds = true
			if tool["outputSchema"] == nil || tool["errorSchema"] == nil {
				t.Fatalf("tool missing schema metadata: %#v", tool)
			}
		}
	}
	if !foundBounds {
		t.Fatal("get_virtual_bounds not found in tool catalog")
	}

	send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_virtual_bounds","arguments":{}}}`)
	callResp := readResp()
	if callResp.Error != nil {
		t.Fatalf("get_virtual_bounds error: %#v", callResp.Error)
	}
	callResult, ok := callResp.Result.(map[string]any)
	if !ok {
		t.Fatalf("unexpected get_virtual_bounds result: %#v", callResp.Result)
	}
	structured, ok := callResult["structuredContent"].(map[string]any)
	if !ok {
		t.Fatalf("missing structuredContent: %#v", callResult)
	}
	for _, field := range []string{"left", "top", "right", "bottom"} {
		if _, ok := structured[field]; !ok {
			t.Fatalf("missing %s in structuredContent: %#v", field, structured)
		}
	}

	send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"click","arguments":{"target_id":"window-1","x":1,"y":1}}}`)
	blockedResp := readResp()
	if blockedResp.Error == nil {
		t.Fatal("expected click to be blocked by default")
	}
	if blockedResp.Error.Data["code"] != "unsafe_operation" {
		t.Fatalf("unexpected blocked error: %#v", blockedResp.Error)
	}
}

func readFramedResponse(reader *bufio.Reader) (*JSONRPCResponse, error) {
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	firstLine = strings.TrimSpace(firstLine)
	if firstLine == "" {
		return readFramedResponse(reader)
	}

	headers := textproto.MIMEHeader{}
	if err := parseHeaders(reader, headers, firstLine); err != nil {
		return nil, err
	}
	lengthHeader := headers.Get("Content-Length")
	var length int
	if _, err := fmt.Sscanf(lengthHeader, "%d", &length); err != nil || length <= 0 {
		return nil, fmt.Errorf("invalid Content-Length header")
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(reader, payload); err != nil {
		return nil, err
	}

	var resp JSONRPCResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
