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

	"github.com/rpdg/winput"
	"github.com/rpdg/winput/screen"
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

func TestReadBoundedLineRejectsOverlongLine(t *testing.T) {
	_, err := readBoundedLine(bufio.NewReader(strings.NewReader(strings.Repeat("A", 9))), 8)
	if err == nil {
		t.Fatal("overlong line accepted")
	}
	got, err := readBoundedLine(bufio.NewReader(strings.NewReader("abc\n")), 8)
	if err != nil || got != "abc\n" {
		t.Fatalf("readBoundedLine = %q, %v", got, err)
	}
}

func TestReadRequestRejectsHugeContentLength(t *testing.T) {
	raw := fmt.Sprintf("Content-Length: %d\r\n\r\n", maxRequestBytes+1)
	_, err := readRequest(bufio.NewReader(strings.NewReader(raw)))
	if err == nil {
		t.Fatal("oversized Content-Length was accepted")
	}
}

func TestReadRequestSkipsBlankLines(t *testing.T) {
	payload := `{"jsonrpc":"2.0","id":1,"method":"ping"}`
	raw := "\r\n\n\n" + payload + "\n"
	req, err := readRequest(bufio.NewReader(strings.NewReader(raw)))
	if err != nil {
		t.Fatalf("readRequest failed: %v", err)
	}
	if req.Method != "ping" {
		t.Fatalf("unexpected method: %s", req.Method)
	}
}

func TestReadRequestRejectsTooManyHeaders(t *testing.T) {
	var raw strings.Builder
	for i := 0; i <= maxHeaderLines; i++ {
		fmt.Fprintf(&raw, "X-%d: y\r\n", i)
	}
	raw.WriteString("\r\n")
	_, err := readRequest(bufio.NewReader(strings.NewReader(raw.String())))
	if err == nil || !strings.Contains(err.Error(), "too many header lines") {
		t.Fatalf("expected header count rejection, got %v", err)
	}
}

func TestReadRequestRejectsOverlongHeaderLine(t *testing.T) {
	raw := "Content-Length: 2\r\nX: " + strings.Repeat("a", maxHeaderBytes) + "\r\n\r\n{}"
	_, err := readRequest(bufio.NewReader(strings.NewReader(raw)))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected header line rejection, got %v", err)
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

func TestPingReturnsEmptyResult(t *testing.T) {
	server := NewServer(Config{})
	resp := server.handleRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("42"),
		Method:  "ping",
	})
	if resp.Error != nil {
		t.Fatalf("ping error: %#v", resp.Error)
	}
	if resp.Result == nil {
		t.Fatal("expected non-nil empty ping result")
	}
}

func TestNegotiateProtocolVersion(t *testing.T) {
	tests := []struct {
		name    string
		offered []string
		want    string
	}{
		{"no offer", nil, defaultProtocolVersion},
		{"empty offer", []string{}, defaultProtocolVersion},
		{"supported single", []string{"2025-03-26"}, "2025-03-26"},
		{"newest wins", []string{"2024-11-05", "2025-06-18"}, "2025-06-18"},
		{"unsupported only", []string{"2025-01-01"}, defaultProtocolVersion},
		{"mixed unsupported and supported", []string{"2025-09-01", "2025-04-01"}, "2025-04-01"},
		{"blank entry ignored", []string{""}, defaultProtocolVersion},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := negotiateProtocolVersion(tt.offered); got != tt.want {
				t.Fatalf("negotiateProtocolVersion(%v) = %q, want %q", tt.offered, got, tt.want)
			}
		})
	}
}

func TestInitializeNegotiatesOfferedVersion(t *testing.T) {
	server := NewServer(Config{})
	resp := server.handleRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2025-03-26"}`),
	})
	if resp.Error != nil {
		t.Fatalf("initialize error: %#v", resp.Error)
	}
	result, ok := resp.Result.(InitializeResult)
	if !ok {
		t.Fatalf("unexpected result: %#v", resp.Result)
	}
	if result.ProtocolVersion != "2025-03-26" {
		t.Fatalf("expected negotiated 2025-03-26, got %q", result.ProtocolVersion)
	}
}

func TestInitializeAcceptsProtocolVersionsArray(t *testing.T) {
	server := NewServer(Config{})
	resp := server.handleRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersions":["2025-03-26","2025-11-25"]}`),
	})
	if resp.Error != nil {
		t.Fatalf("initialize error: %#v", resp.Error)
	}
	result, ok := resp.Result.(InitializeResult)
	if !ok {
		t.Fatalf("unexpected result: %#v", resp.Result)
	}
	if result.ProtocolVersion != "2025-11-25" {
		t.Fatalf("expected negotiated 2025-11-25, got %q", result.ProtocolVersion)
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

func TestCatalogToolContract(t *testing.T) {
	catalog := defaultCatalog()
	if len(catalog) != 19 {
		t.Fatalf("catalog has %d tools, want 19", len(catalog))
	}
	seen := map[string]bool{}
	for _, tool := range catalog {
		if tool.Name == "" || tool.Description == "" {
			t.Fatalf("tool with empty name or description: %#v", tool)
		}
		if seen[tool.Name] {
			t.Fatalf("duplicate tool name %q", tool.Name)
		}
		seen[tool.Name] = true
		if tool.InputSchema == nil || tool.OutputSchema == nil || tool.ErrorSchema == nil {
			t.Fatalf("%s missing a schema", tool.Name)
		}
		if _, ok := tool.InputSchema["properties"]; !ok {
			t.Fatalf("%s input schema missing properties", tool.Name)
		}
		if got := tool.Annotations["sideEffectLevel"]; got != "read_only" && got != "state_change" && got != "sensitive" && got != "destructive" {
			t.Fatalf("%s has invalid sideEffectLevel %#v", tool.Name, got)
		}
		if _, ok := tool.Annotations["requiresConfirmation"].(bool); !ok {
			t.Fatalf("%s missing boolean requiresConfirmation", tool.Name)
		}
	}
}

func TestToolsListIncludesSchemas(t *testing.T) {
	server := NewServer(Config{})
	resp := server.handleRequest(context.Background(), JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      json.RawMessage("1"),
		Method:  "tools/list",
	})

	result, ok := resp.Result.(ToolListResult)
	if !ok {
		t.Fatalf("unexpected result type: %#v", resp.Result)
	}
	toolsRaw := result.Tools
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
	initResult, ok := initResp.Result.(InitializeResult)
	if !ok {
		t.Fatalf("unexpected initialize result: %#v", initResp.Result)
	}
	if initResult.ProtocolVersion != "2025-04-01" {
		t.Fatalf("unexpected protocolVersion: %#v", initResult.ProtocolVersion)
	}

	send(`{"jsonrpc":"2.0","id":9,"method":"ping"}`)
	pingResp := readResp()
	if pingResp.Error != nil {
		t.Fatalf("ping error: %#v", pingResp.Error)
	}
	if pingResp.Result == nil {
		t.Fatal("expected empty ping result")
	}

	send(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	listResp := readResp()
	if listResp.Error != nil {
		t.Fatalf("tools/list error: %#v", listResp.Error)
	}
	listResult, ok := listResp.Result.(ToolListResult)
	if !ok {
		t.Fatalf("unexpected tools/list result: %#v", listResp.Result)
	}
	toolsAny := listResult.Tools
	if len(toolsAny) == 0 {
		t.Fatalf("unexpected tools payload: %#v", listResult.Tools)
	}
	var foundBounds bool
	for _, tool := range toolsAny {
		if tool.Name == "get_virtual_bounds" {
			foundBounds = true
			if tool.OutputSchema == nil || tool.ErrorSchema == nil {
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
	callResult, ok := callResp.Result.(ToolCallResult[VirtualBoundsResult])
	if !ok {
		t.Fatalf("unexpected get_virtual_bounds result: %#v", callResp.Result)
	}
	if callResult.StructuredContent.Right == 0 && callResult.StructuredContent.Bottom == 0 {
		t.Fatalf("unexpected structuredContent: %#v", callResult.StructuredContent)
	}

	send(`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"click","arguments":{"target_id":"window-1","x":1,"y":1}}}`)
	blockedResp := readResp()
	if blockedResp.Error == nil {
		t.Fatal("expected click to be blocked by default")
	}
	if blockedResp.Error.Data == nil || blockedResp.Error.Data.Code != "unsafe_operation" {
		t.Fatalf("unexpected blocked error: %#v", blockedResp.Error)
	}
}

func TestPickCandidate(t *testing.T) {
	wins := []*winput.Window{{HWND: 100}, {HWND: 200}}
	if got, err := pickCandidate(wins, 0); err != nil || got.HWND != 100 {
		t.Fatalf("pickCandidate(wins, 0) = %#v, %v", got, err)
	}
	if got, err := pickCandidate(wins, 1); err != nil || got.HWND != 200 {
		t.Fatalf("pickCandidate(wins, 1) = %#v, %v", got, err)
	}
	for _, idx := range []int32{-1, 2} {
		if _, err := pickCandidate(wins, idx); err == nil {
			t.Fatalf("expected error for index %d", idx)
		}
	}
}

func TestInsideVirtualBoundsRule(t *testing.T) {
	rect := screen.Rect{Left: -1920, Top: 0, Right: 0, Bottom: 1080}
	inside := [][2]int32{{-1920, 0}, {-1, 1079}, {-1920, 1079}}
	for _, p := range inside {
		if !insideVirtualBoundsRule(p[0], p[1], rect) {
			t.Fatalf("%v should be inside", p)
		}
	}
	outside := [][2]int32{{-1921, 0}, {-1920, -1}, {0, 1080}, {-1, 1080}}
	for _, p := range outside {
		if insideVirtualBoundsRule(p[0], p[1], rect) {
			t.Fatalf("%v should be outside", p)
		}
	}
}

func TestResolveClickAt(t *testing.T) {
	for _, button := range []string{"", "left", "right", "middle"} {
		if _, err := resolveClickAt(button); err != nil {
			t.Fatalf("resolveClickAt(%q) error: %v", button, err)
		}
	}
	if _, err := resolveClickAt("bogus"); err == nil {
		t.Fatal("expected error for unsupported button")
	}
}

func TestNewScreenToolsRequireCoordinates(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	for _, name := range []string{"move_mouse_to", "click_at", "double_click_at"} {
		t.Run(name, func(t *testing.T) {
			_, err := server.callTool(context.Background(), ToolCallParams{
				Name:      name,
				Arguments: map[string]any{"x": 100}, // y missing
			})
			assertValidationError(t, err, "missing required argument: y")
		})
	}
}

func TestRequireArgsPresenceSemantics(t *testing.T) {
	if err := requireArgs(map[string]any{"x": 0, "y": 0, "delta": 0}, "x", "y", "delta"); err != nil {
		t.Fatalf("explicit zero must be accepted: %v", err)
	}
	cases := []struct {
		args map[string]any
		keys []string
	}{
		{map[string]any{}, []string{"x", "y"}},
		{map[string]any{"x": nil, "y": 0}, []string{"x", "y"}},
		{map[string]any{"x": 0}, []string{"x", "y"}},
		{map[string]any{"x": 0, "y": 0, "delta": nil}, []string{"x", "y", "delta"}},
	}
	for _, c := range cases {
		if err := requireArgs(c.args, c.keys...); err == nil {
			t.Fatalf("requireArgs(%v, %v) accepted missing or null argument", c.args, c.keys)
		}
	}
}

func TestNewScreenToolsRejectNullCoordinates(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	for _, name := range []string{"move_mouse_to", "click_at", "double_click_at"} {
		t.Run(name, func(t *testing.T) {
			for _, args := range []map[string]any{
				{"x": nil, "y": 0},
				{"x": 0, "y": nil},
				{"x": nil, "y": nil},
			} {
				_, err := server.callTool(context.Background(), ToolCallParams{
					Name:      name,
					Arguments: args,
				})
				assertValidationError(t, err, "missing required argument")
			}
		})
	}
}

func TestScrollRejectsNullArguments(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	for _, args := range []map[string]any{
		{"target_id": "window-1", "x": nil, "y": 0, "delta": 120},
		{"target_id": "window-1", "x": 0, "y": nil, "delta": 120},
		{"target_id": "window-1", "x": 0, "y": 0, "delta": nil},
	} {
		_, err := server.callTool(context.Background(), ToolCallParams{
			Name:      "scroll",
			Arguments: args,
		})
		assertValidationError(t, err, "missing required argument")
	}
}

func TestScrollAcceptsExplicitZeroDelta(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	_, err := server.callTool(context.Background(), ToolCallParams{
		Name:      "scroll",
		Arguments: map[string]any{"target_id": "window-1", "x": 0, "y": 0, "delta": 0},
	})
	// Presence validation accepts zero; the unknown target is the next
	// failure, which proves no input was sent.
	assertCodedError(t, err, "target_not_found", "")
}

func TestScrollRequiresDelta(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	_, err := server.callTool(context.Background(), ToolCallParams{
		Name:      "scroll",
		Arguments: map[string]any{"target_id": "window-1", "x": 1, "y": 1},
	})
	assertValidationError(t, err, "missing required argument: delta")
}

func TestScrollWheelMissingStepsRejectedByLibrary(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	_, err := server.callTool(context.Background(), ToolCallParams{
		Name:      "scroll_wheel",
		Arguments: map[string]any{},
	})
	// Missing steps decodes to 0; the library rejects 0 itself (Q13).
	assertValidationError(t, err, "")
}

func TestPressHotkeyRejectsEmptyOrNullKeys(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	for _, args := range []map[string]any{
		{"target_id": "window-1"},
		{"target_id": "window-1", "keys": nil},
		{"target_id": "window-1", "keys": []string{}},
		{"target_id": "window-1", "keys": []any{}},
	} {
		_, err := server.callTool(context.Background(), ToolCallParams{
			Name:      "press_hotkey",
			Arguments: args,
		})
		assertValidationError(t, err, "")
	}
}

func TestNewStateChangeToolsBlockedByDefault(t *testing.T) {
	server := NewServer(Config{})
	for _, name := range []string{"move_mouse_to", "click_at", "double_click_at", "scroll_wheel", "scroll"} {
		t.Run(name, func(t *testing.T) {
			_, err := server.callTool(context.Background(), ToolCallParams{Name: name})
			assertCodedError(t, err, "unsafe_operation", "mutating tools are disabled")
		})
	}
}

func TestPressHotkeyMaxKeys(t *testing.T) {
	server := NewServer(Config{AllowMutations: true})
	nine := make([]string, 9)
	for i := range nine {
		nine[i] = "A"
	}
	_, err := server.callTool(context.Background(), ToolCallParams{
		Name:      "press_hotkey",
		Arguments: map[string]any{"target_id": "window-1", "keys": nine},
	})
	assertValidationError(t, err, "too many keys")

	eight := make([]string, 8)
	for i := range eight {
		eight[i] = "A"
	}
	_, err = server.callTool(context.Background(), ToolCallParams{
		Name:      "press_hotkey",
		Arguments: map[string]any{"target_id": "window-1", "keys": eight},
	})
	// Cap passes; the next step is the registry lookup -> target_not_found.
	var coded *codedError
	if !errors.As(err, &coded) || coded.Code != "target_not_found" {
		t.Fatalf("expected target_not_found after cap passes, got %#v", err)
	}
}

func TestPidRejectsOutOfRange(t *testing.T) {
	server := NewServer(Config{})
	for _, args := range []map[string]any{
		{"pid": -1},
		{"pid": 1.5},
		{"pid": 5000000000.0}, // exceeds uint32
		{"title": "X", "pid": -1},
	} {
		t.Run(fmt.Sprintf("%v", args), func(t *testing.T) {
			_, err := server.callTool(context.Background(), ToolCallParams{
				Name:      "find_window",
				Arguments: args,
			})
			assertValidationError(t, err, "")
		})
	}
}

func TestFindWindowPIDZeroMeansUnused(t *testing.T) {
	_, _, err := findWindow(FindWindowArgs{PID: 0})
	assertValidationError(t, err, "one of title")

	prev := findTitle
	findTitle = func(string) (*winput.Window, error) { return &winput.Window{HWND: 7}, nil }
	defer func() { findTitle = prev }()

	win, count, err := findWindow(FindWindowArgs{Title: "X", PID: 0})
	if err != nil || win == nil || win.HWND != 7 || count != 0 {
		t.Fatalf("title with pid 0: win=%#v count=%d err=%v", win, count, err)
	}
}

func TestKeyFromNameShortNames(t *testing.T) {
	table := []struct {
		name string
		want winput.Key
	}{
		{"A", winput.KeyA}, {"B", winput.KeyB}, {"C", winput.KeyC}, {"D", winput.KeyD},
		{"E", winput.KeyE}, {"F", winput.KeyF}, {"G", winput.KeyG}, {"H", winput.KeyH},
		{"I", winput.KeyI}, {"J", winput.KeyJ}, {"K", winput.KeyK}, {"L", winput.KeyL},
		{"M", winput.KeyM}, {"N", winput.KeyN}, {"O", winput.KeyO}, {"P", winput.KeyP},
		{"Q", winput.KeyQ}, {"R", winput.KeyR}, {"S", winput.KeyS}, {"T", winput.KeyT},
		{"U", winput.KeyU}, {"V", winput.KeyV}, {"W", winput.KeyW}, {"X", winput.KeyX},
		{"Y", winput.KeyY}, {"Z", winput.KeyZ},
		{"0", winput.Key0}, {"1", winput.Key1}, {"2", winput.Key2}, {"3", winput.Key3},
		{"4", winput.Key4}, {"5", winput.Key5}, {"6", winput.Key6}, {"7", winput.Key7},
		{"8", winput.Key8}, {"9", winput.Key9},
		{"ESC", winput.KeyEsc}, {"TAB", winput.KeyTab}, {"SPACE", winput.KeySpace}, {"BKSP", winput.KeyBkSp},
		{"F1", winput.KeyF1}, {"F2", winput.KeyF2}, {"F3", winput.KeyF3}, {"F4", winput.KeyF4},
		{"F5", winput.KeyF5}, {"F6", winput.KeyF6}, {"F7", winput.KeyF7}, {"F8", winput.KeyF8},
		{"F9", winput.KeyF9}, {"F10", winput.KeyF10}, {"F11", winput.KeyF11}, {"F12", winput.KeyF12},
		{"CAPS", winput.KeyCaps}, {"NUMLOCK", winput.KeyNumLock}, {"SCROLLLOCK", winput.KeyScroll},
		{"CTRL", winput.KeyCtrl}, {"SHIFT", winput.KeyShift}, {"ALT", winput.KeyAlt},
		{"RIGHTCTRL", winput.KeyRightCtrl}, {"RIGHTSHIFT", winput.KeyRightShift}, {"RIGHTALT", winput.KeyRightAlt},
		{"LWIN", winput.KeyLWin}, {"RWIN", winput.KeyRWin},
		{"UP", winput.KeyArrowUp}, {"DOWN", winput.KeyArrowDown}, {"LEFT", winput.KeyLeft}, {"RIGHT", winput.KeyRight},
		{"HOME", winput.KeyHome}, {"END", winput.KeyEnd}, {"INSERT", winput.KeyInsert}, {"DELETE", winput.KeyDelete},
		{"PAGEUP", winput.KeyPageUp}, {"PAGEDOWN", winput.KeyPageDown},
		{"MINUS", winput.KeyMinus}, {"EQUAL", winput.KeyEqual}, {"COMMA", winput.KeyComma}, {"DOT", winput.KeyDot},
		{"SLASH", winput.KeySlash}, {"SEMI", winput.KeySemi}, {"QUOT", winput.KeyQuot}, {"TICK", winput.KeyTick},
		{"LBR", winput.KeyLBr}, {"RBR", winput.KeyRBr}, {"BACKSLASH", winput.KeyBackslash},
		{"NUMPAD0", winput.KeyNumpad0}, {"NUMPAD1", winput.KeyNumpad1}, {"NUMPAD2", winput.KeyNumpad2},
		{"NUMPAD3", winput.KeyNumpad3}, {"NUMPAD4", winput.KeyNumpad4}, {"NUMPAD5", winput.KeyNumpad5},
		{"NUMPAD6", winput.KeyNumpad6}, {"NUMPAD7", winput.KeyNumpad7}, {"NUMPAD8", winput.KeyNumpad8},
		{"NUMPAD9", winput.KeyNumpad9}, {"NUMPADDECIMAL", winput.KeyNumpadDecimal},
		{"NUMPADMULTIPLY", winput.KeyNumpadMultiply}, {"NUMPADADD", winput.KeyNumpadAdd},
		{"NUMPADSUBTRACT", winput.KeyNumpadSubtract}, {"NUMPADDIVIDE", winput.KeyNumpadDivide},
		{"NUMPADENTER", winput.KeyNumpadEnter},
		{"ENTER", winput.KeyEnter},
	}
	for _, tt := range table {
		got, err := keyFromName(tt.name)
		if err != nil {
			t.Fatalf("keyFromName(%q): %v", tt.name, err)
		}
		if got != tt.want {
			t.Fatalf("keyFromName(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
	if len(keyNames) != len(table) {
		t.Fatalf("keyNames has %d entries, exhaustive contract test has %d", len(keyNames), len(table))
	}
	for name, want := range keyNames {
		got, err := keyFromName(strings.ToLower(name))
		if err != nil || got != want {
			t.Fatalf("keyFromName(%q) = %v, %v; want %v", strings.ToLower(name), got, err, want)
		}
	}
	for _, variant := range []struct {
		name string
		want winput.Key
	}{
		{"enter", winput.KeyEnter},
		{" Enter ", winput.KeyEnter},
	} {
		got, err := keyFromName(variant.name)
		if err != nil || got != variant.want {
			t.Fatalf("keyFromName(%q) = %v, %v; want %v", variant.name, got, err, variant.want)
		}
	}
}

func TestKeyFromNameSingleNonShiftChars(t *testing.T) {
	chars := map[rune]winput.Key{
		',': winput.KeyComma, '.': winput.KeyDot, '-': winput.KeyMinus, ';': winput.KeySemi,
		'[': winput.KeyLBr, ']': winput.KeyRBr, '\\': winput.KeyBackslash,
		'\'': winput.KeyQuot, '`': winput.KeyTick, '=': winput.KeyEqual, '/': winput.KeySlash,
	}
	for r, want := range chars {
		got, err := keyFromName(string(r))
		if err != nil {
			t.Fatalf("keyFromName(%q): %v", string(r), err)
		}
		if got != want {
			t.Fatalf("keyFromName(%q) = %v, want %v", string(r), got, want)
		}
	}
}

func TestKeyFromNameRejectsShiftCharsAndUnknown(t *testing.T) {
	for _, name := range []string{"!", "?", "<", ":", "\"", "FRED", "", "  ", "CTRLSOMETHING"} {
		if _, err := keyFromName(name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
}

func TestMapLibraryErrorScrollMappings(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{winput.ErrInvalidScrollSteps, "validation_error"},
		{winput.ErrInvalidScrollDelta, "validation_error"},
		{winput.ErrInvalidMouseCoordinate, "validation_error"},
		{winput.ErrUnsupportedKey, "validation_error"},
		{winput.ErrOutsideVirtualDesktop, "validation_error"},
		{winput.ErrDirectedScrollUnsupported, "backend_unavailable"},
		{winput.ErrWindowNotFound, "target_not_found"},
		{winput.ErrWindowGone, "target_not_found"},
		{winput.ErrWindowNotVisible, "target_not_visible"},
		{winput.ErrPermissionDenied, "permission_denied"},
	}
	for _, c := range cases {
		var coded *codedError
		if !errors.As(mapLibraryError(c.err), &coded) || coded.Code != c.want {
			t.Fatalf("mapLibraryError(%v) = code, want %s", c.err, c.want)
		}
	}
}

func TestFindWindowSchemaIncludesNewFields(t *testing.T) {
	catalog := defaultCatalog()
	var fw Tool
	for _, tool := range catalog {
		if tool.Name == "find_window" {
			fw = tool
			break
		}
	}
	if fw.Name == "" {
		t.Fatal("find_window missing from catalog")
	}
	iprops := fw.InputSchema["properties"].(map[string]any)
	for _, k := range []string{"pid", "index"} {
		if _, ok := iprops[k]; !ok {
			t.Fatalf("find_window input missing %q", k)
		}
	}
	oprops := fw.OutputSchema["properties"].(map[string]any)
	if _, ok := oprops["count"]; !ok {
		t.Fatal("find_window output missing count")
	}
}

func TestNewToolsInCatalog(t *testing.T) {
	catalog := defaultCatalog()
	names := map[string]bool{}
	for _, tool := range catalog {
		names[tool.Name] = true
	}
	for _, name := range []string{"move_mouse_to", "click_at", "double_click_at", "scroll_wheel", "scroll"} {
		if !names[name] {
			t.Fatalf("tool %q missing from catalog", name)
		}
	}
}

func TestPressHotkeySchemaLimits(t *testing.T) {
	ph := toolByName(t, "press_hotkey")
	keys := ph.InputSchema["properties"].(map[string]any)["keys"].(map[string]any)
	if got := keys["maxItems"]; got != 8 {
		t.Fatalf("maxItems = %#v, want 8", got)
	}
	if got := keys["minItems"]; got != 1 {
		t.Fatalf("minItems = %#v, want 1", got)
	}
}

func TestNewToolSchemas(t *testing.T) {
	cases := []struct {
		name     string
		required []string
	}{
		{"move_mouse_to", []string{"x", "y"}},
		{"click_at", []string{"x", "y"}},
		{"double_click_at", []string{"x", "y"}},
		{"scroll_wheel", []string{"steps"}},
		{"scroll", []string{"target_id", "x", "y", "delta"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tool := toolByName(t, c.name)
			if tool.OutputSchema == nil || tool.ErrorSchema == nil {
				t.Fatalf("%s missing outputSchema/errorSchema", c.name)
			}
			if got := tool.Annotations["sideEffectLevel"]; got != "state_change" {
				t.Fatalf("%s sideEffectLevel = %#v, want state_change", c.name, got)
			}
			if got := tool.Annotations["requiresConfirmation"]; got != true {
				t.Fatalf("%s requiresConfirmation = %#v, want true", c.name, got)
			}
			required := schemaRequired(t, tool.InputSchema)
			for _, field := range c.required {
				if !required[field] {
					t.Fatalf("%s missing required field %q in %v", c.name, field, required)
				}
			}
			if c.name == "scroll" {
				for _, want := range []string{"message backend", "backend_unavailable", "120 per notch"} {
					if !strings.Contains(tool.Description, want) {
						t.Fatalf("scroll description missing %q: %s", want, tool.Description)
					}
				}
			}
		})
	}
}

func TestFindWindowSchemaAndDescription(t *testing.T) {
	fw := toolByName(t, "find_window")
	iprops := fw.InputSchema["properties"].(map[string]any)
	for _, k := range []string{"title", "class", "process_name", "pid", "index"} {
		if _, ok := iprops[k]; !ok {
			t.Fatalf("find_window input missing %q", k)
		}
	}
	oprops := fw.OutputSchema["properties"].(map[string]any)
	for _, k := range []string{"target_id", "kind", "count", "is_valid", "is_visible"} {
		if _, ok := oprops[k]; !ok {
			t.Fatalf("find_window output missing %q", k)
		}
	}
	for _, want := range []string{"title > class > process_name > pid", "Z-order", "every process", "read_text"} {
		if !strings.Contains(fw.Description, want) {
			t.Fatalf("find_window description missing %q: %s", want, fw.Description)
		}
	}
}

func toolByName(t *testing.T, name string) Tool {
	t.Helper()
	for _, tool := range defaultCatalog() {
		if tool.Name == name {
			return tool
		}
	}
	t.Fatalf("tool %q missing from catalog", name)
	return Tool{}
}

func schemaRequired(t *testing.T, schema map[string]any) map[string]bool {
	t.Helper()
	required := map[string]bool{}
	raw, ok := schema["required"]
	if !ok {
		return required
	}
	fields, ok := raw.([]string)
	if !ok {
		t.Fatalf("required is %T, want []string", raw)
	}
	for _, field := range fields {
		required[field] = true
	}
	return required
}

func TestFindWindowTitleIndexRejected(t *testing.T) {
	prev := findTitle
	findTitle = func(string) (*winput.Window, error) { return &winput.Window{HWND: 42}, nil }
	defer func() { findTitle = prev }()

	win, count, err := findWindow(FindWindowArgs{Title: "X", Index: 0})
	if err != nil || win == nil || win.HWND != 42 || count != 0 {
		t.Fatalf("title index 0: win=%#v count=%d err=%v", win, count, err)
	}
	for _, idx := range []int32{2, -1} {
		_, _, err = findWindow(FindWindowArgs{Title: "X", Index: idx})
		assertCodedError(t, err, "validation_error", "not allowed for title search")
	}
}

func TestFindWindowLookupRunsBeforeIndexCheck(t *testing.T) {
	prev := findTitle
	findTitle = func(string) (*winput.Window, error) { return nil, winput.ErrWindowNotFound }
	defer func() { findTitle = prev }()

	_, _, err := findWindow(FindWindowArgs{Title: "X", Index: 2})
	// The lookup runs first: a missing window is target_not_found before
	// the index rule is consulted.
	assertCodedError(t, err, "target_not_found", "")
}

func TestFindWindowProcessNameIndexAndCount(t *testing.T) {
	prev := findByName
	defer func() { findByName = prev }()
	findByName = func(string) ([]*winput.Window, error) {
		return []*winput.Window{{HWND: 10}, {HWND: 20}}, nil
	}

	win, count, err := findWindow(FindWindowArgs{ProcessName: "app.exe", Index: 1})
	if err != nil || win == nil || win.HWND != 20 || count != 2 {
		t.Fatalf("got win=%#v count=%d err=%v", win, count, err)
	}
	_, _, err = findWindow(FindWindowArgs{ProcessName: "app.exe", Index: 2})
	assertCodedError(t, err, "validation_error", "out of range")

	findByName = func(string) ([]*winput.Window, error) { return nil, nil }
	_, _, err = findWindow(FindWindowArgs{ProcessName: "gone.exe"})
	assertCodedError(t, err, "target_not_found", "")
}

func TestFindWindowPIDIndexAndCount(t *testing.T) {
	prev := findByPID
	findByPID = func(uint32) ([]*winput.Window, error) {
		return []*winput.Window{{HWND: 1}, {HWND: 2}}, nil
	}
	defer func() { findByPID = prev }()

	win, count, err := findWindow(FindWindowArgs{PID: 1234, Index: 1})
	if err != nil || win == nil || win.HWND != 2 || count != 2 {
		t.Fatalf("got win=%#v count=%d err=%v", win, count, err)
	}
}

func TestKeyDescriptionsExposeFullTable(t *testing.T) {
	catalog := defaultCatalog()
	var pressKey, pressHotkey string
	for _, tool := range catalog {
		switch tool.Name {
		case "press_key":
			pressKey = tool.Description
		case "press_hotkey":
			pressHotkey = tool.Description
		}
	}
	if pressKey == "" || pressHotkey == "" {
		t.Fatal("press_key/press_hotkey missing descriptions")
	}
	for name, desc := range map[string]string{"press_key": pressKey, "press_hotkey": pressHotkey} {
		for _, s := range []string{"BKSP", "RIGHTCTRL", "SCROLLLOCK", "NUMPADDIVIDE", "NUMPADENTER", "LWIN", "F12", "PAGEUP"} {
			if !strings.Contains(desc, s) {
				t.Fatalf("%s description missing %q", name, s)
			}
		}
		// Rejected aliases must not appear as standalone tokens
		// (NUMPADDIVIDE legitimately contains "DIVIDE" as a substring).
		for _, alias := range []string{"BACKSPACE", "ESCAPE", "ARROWUP", "LEFTCTRL", "DIVIDE"} {
			for _, tok := range strings.FieldsFunc(desc, func(r rune) bool {
				return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
			}) {
				if strings.EqualFold(tok, alias) {
					t.Fatalf("%s description must not contain alias %q", name, alias)
				}
			}
		}
	}
}

func assertCodedError(t *testing.T, err error, wantCode, wantContains string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %s", wantCode)
	}
	var coded *codedError
	if !errors.As(err, &coded) || coded.Code != wantCode {
		t.Fatalf("expected code %s, got %#v", wantCode, err)
	}
	if wantContains != "" && !strings.Contains(err.Error(), wantContains) {
		t.Fatalf("error %q is missing %q", err.Error(), wantContains)
	}
}

func assertValidationError(t *testing.T, err error, wantContains string) {
	t.Helper()
	assertCodedError(t, err, "validation_error", wantContains)
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
	resp.Result = decodeKnownResult(resp.Result)
	return &resp, nil
}

func decodeKnownResult(v any) any {
	switch data := v.(type) {
	case map[string]any:
		if _, ok := data["protocolVersion"]; ok {
			buf, _ := json.Marshal(data)
			var out InitializeResult
			_ = json.Unmarshal(buf, &out)
			return out
		}
		if _, ok := data["tools"]; ok {
			buf, _ := json.Marshal(data)
			var out ToolListResult
			_ = json.Unmarshal(buf, &out)
			return out
		}
		if structured, ok := data["structuredContent"].(map[string]any); ok {
			if _, ok := structured["left"]; ok {
				buf, _ := json.Marshal(data)
				var out ToolCallResult[VirtualBoundsResult]
				_ = json.Unmarshal(buf, &out)
				return out
			}
		}
	}
	return v
}
