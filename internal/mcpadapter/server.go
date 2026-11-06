package mcpadapter

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rpdg/winput"
	"github.com/rpdg/winput/keyboard"
	"github.com/rpdg/winput/screen"
)

type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      json.RawMessage  `json:"id,omitempty"`
	Result  any              `json:"result,omitempty"`
	Error   *JSONRPCErrorObj `json:"error,omitempty"`
}

type Server struct {
	catalog        []Tool
	registry       *Registry
	allowMutations bool
	allowSensitive bool
}

type Config struct {
	AllowMutations bool
	AllowSensitive bool
}

func NewServer(cfg Config) *Server {
	reg := NewRegistry()
	return &Server{
		catalog:        defaultCatalog(),
		registry:       reg,
		allowMutations: cfg.AllowMutations,
		allowSensitive: cfg.AllowSensitive,
	}
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := readRequest(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			if writeErr := writeResponse(out, JSONRPCResponse{
				JSONRPC: "2.0",
				Error: &JSONRPCErrorObj{
					Code:    -32700,
					Message: "parse error",
					Data:    &JSONRPCErrorData{Details: err.Error()},
				},
			}); writeErr != nil {
				return writeErr
			}
			continue
		}

		resp := s.handleRequest(ctx, *req)
		if req.ID == nil {
			continue
		}
		if err := writeResponse(out, resp); err != nil {
			return err
		}
	}
}

func (s *Server) handleRequest(ctx context.Context, req JSONRPCRequest) JSONRPCResponse {
	resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		var params InitializeParams
		// Malformed params fall back to the default protocol version.
		_ = decodeParams(req.Params, &params)
		offered := params.ProtocolVersions
		if params.ProtocolVersion != "" {
			offered = append(offered, params.ProtocolVersion)
		}
		resp.Result = InitializeResult{
			ProtocolVersion: negotiateProtocolVersion(offered),
			ServerInfo: ServerInfo{
				Name:    "winput-mcp",
				Version: "0.1.0",
			},
			Capabilities: map[string]any{
				"tools": map[string]any{},
			},
		}
		return resp
	case "ping":
		resp.Result = map[string]any{}
		return resp
	case "notifications/initialized":
		return resp
	case "tools/list":
		resp.Result = ToolListResult{Tools: s.catalog}
		return resp
	case "tools/call":
		var params ToolCallParams
		if err := decodeParams(req.Params, &params); err != nil {
			resp.Error = invalidParams(err)
			return resp
		}
		result, err := s.callTool(ctx, params)
		if err != nil {
			resp.Error = toJSONRPCError(err)
			return resp
		}
		resp.Result = result
		return resp
	default:
		resp.Error = &JSONRPCErrorObj{Code: -32601, Message: "method not found"}
		return resp
	}
}

// defaultProtocolVersion is reported when the client offers no version this
// server honors. It matches the version the server previously returned.
const defaultProtocolVersion = "2025-04-01"

// supportedProtocolVersions are the protocol revisions this tool-only server
// can honor, oldest to newest. Tool discovery, tool calls, and ping are stable
// across all of them.
var supportedProtocolVersions = []string{
	"2024-11-05",
	"2025-03-26",
	"2025-04-01",
	"2025-06-18",
	"2025-11-25",
}

// negotiateProtocolVersion returns the newest client-offered version this
// server supports, falling back to defaultProtocolVersion when there is no
// overlap.
func negotiateProtocolVersion(offered []string) string {
	has := make(map[string]struct{}, len(offered))
	for _, v := range offered {
		if v != "" {
			has[v] = struct{}{}
		}
	}
	for i := len(supportedProtocolVersions) - 1; i >= 0; i-- {
		if _, ok := has[supportedProtocolVersions[i]]; ok {
			return supportedProtocolVersions[i]
		}
	}
	return defaultProtocolVersion
}

func (s *Server) callTool(ctx context.Context, params ToolCallParams) (any, error) {
	if err := s.checkToolAccess(params.Name); err != nil {
		return nil, err
	}

	switch params.Name {
	case "find_window":
		var args FindWindowArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		win, count, err := findWindow(args)
		if err != nil {
			return nil, err
		}
		target := s.registry.AddWindow(win, args.alias())
		result := describeWindowTarget(target, win)
		result.Title = args.Title
		result.Class = args.Class
		result.ProcessName = args.ProcessName
		if count > 0 {
			result.Count = int32(count)
		}
		return toolResult(result), nil
	case "find_child":
		var args FindChildArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		parent, err := s.registry.Window(args.ParentTargetID)
		if err != nil {
			return nil, err
		}
		child, err := parent.FindChildByClass(args.Class)
		if err != nil {
			return nil, targetNotFound(err)
		}
		target := s.registry.AddWindow(child, args.Class)
		result := describeWindowTarget(target, child)
		result.Class = args.Class
		result.ParentTargetID = args.ParentTargetID
		return toolResult(result), nil
	case "click":
		var args ClickArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		switch args.Button {
		case "", "left":
			err = target.Click(args.X, args.Y)
		case "right":
			err = target.ClickRight(args.X, args.Y)
		case "middle":
			err = target.ClickMiddle(args.X, args.Y)
		default:
			return nil, invalidRequestError("validation_error", fmt.Errorf("unsupported button: %s", args.Button))
		}
		if err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "double_click":
		var args ClickArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		if err := target.DoubleClick(args.X, args.Y); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "right_click":
		var args ClickArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		if err := target.ClickRight(args.X, args.Y); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "move_mouse":
		var args ClickArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		if err := target.Move(args.X, args.Y); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "type_text":
		var args TypeTextArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		if err := target.Type(args.Text); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "press_key":
		var args PressKeyArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		key, err := keyFromName(args.Key)
		if err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		if err := target.Press(key); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "press_hotkey":
		var args PressHotkeyArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if len(args.Keys) > maxPressHotkeyKeys {
			return nil, invalidRequestError("validation_error", fmt.Errorf("too many keys: %d (max %d)", len(args.Keys), maxPressHotkeyKeys))
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		keys := make([]winput.Key, 0, len(args.Keys))
		for _, name := range args.Keys {
			key, err := keyFromName(name)
			if err != nil {
				return nil, invalidRequestError("validation_error", err)
			}
			keys = append(keys, key)
		}
		if err := target.PressHotkey(keys...); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "read_text":
		var args ReadTextArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		var text string
		if args.BestEffort {
			text, err = target.Value()
		} else {
			text, err = target.Text()
		}
		if err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(ReadTextResult{Text: text}), nil
	case "capture_screen":
		img, err := screen.CaptureVirtualDesktop()
		if err != nil {
			return nil, mapLibraryError(err)
		}
		bounds := img.Bounds()
		return toolResult(CaptureScreenResult{Width: bounds.Dx(), Height: bounds.Dy()}), nil
	case "switch_backend":
		var args SwitchBackendArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		var backend winput.Backend
		switch args.Backend {
		case "message":
			backend = winput.BackendMessage
		case "hid":
			backend = winput.BackendHID
		default:
			return nil, invalidRequestError("validation_error", fmt.Errorf("unsupported backend: %s", args.Backend))
		}
		if err := winput.SetBackend(backend); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(SwitchBackendResult{OK: true, Backend: args.Backend}), nil
	case "get_cursor_pos":
		x, y, err := winput.GetCursorPos()
		if err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(CursorPosResult{X: x, Y: y}), nil
	case "get_virtual_bounds":
		rect := screen.VirtualBounds()
		return toolResult(VirtualBoundsResult{Left: rect.Left, Top: rect.Top, Right: rect.Right, Bottom: rect.Bottom}), nil
	case "move_mouse_to":
		var args ScreenPointArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := requireArgs(params.Arguments, "x", "y"); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := checkVirtualPoint(args.X, args.Y); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := winput.MoveMouseTo(args.X, args.Y); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "click_at":
		var args ClickAtArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := requireArgs(params.Arguments, "x", "y"); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := checkVirtualPoint(args.X, args.Y); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		click, err := resolveClickAt(args.Button)
		if err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := click(args.X, args.Y); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "double_click_at":
		var args ScreenPointArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := requireArgs(params.Arguments, "x", "y"); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := checkVirtualPoint(args.X, args.Y); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := winput.DoubleClickMouseAt(args.X, args.Y); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "scroll_wheel":
		var args ScrollWheelArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		// Missing steps decodes to 0 and the library rejects 0 itself
		// (Q13); no explicit presence check needed.
		if err := winput.ScrollWheel(args.Steps); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	case "scroll":
		var args ScrollArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		if err := requireArgs(params.Arguments, "x", "y", "delta"); err != nil {
			return nil, invalidRequestError("validation_error", err)
		}
		target, err := s.registry.Window(args.TargetID)
		if err != nil {
			return nil, err
		}
		if err := target.Scroll(args.X, args.Y, args.Delta); err != nil {
			return nil, mapLibraryError(err)
		}
		return toolResult(OKResult{OK: true}), nil
	default:
		return nil, invalidRequestError("validation_error", fmt.Errorf("unknown tool: %s", params.Name))
	}
}

func (s *Server) checkToolAccess(name string) error {
	sideEffect := "read_only"
	for _, tool := range s.catalog {
		if tool.Name == name {
			if value, ok := tool.Annotations["sideEffectLevel"].(string); ok {
				sideEffect = value
			}
			break
		}
	}

	switch sideEffect {
	case "state_change":
		if !s.allowMutations {
			return &codedError{
				Code:      "unsafe_operation",
				Message:   "mutating tools are disabled",
				Retryable: false,
				Cause:     fmt.Errorf("restart server with mutation permission enabled"),
			}
		}
	case "sensitive", "destructive":
		if !s.allowSensitive {
			return &codedError{
				Code:      "unsafe_operation",
				Message:   "sensitive tools are disabled",
				Retryable: false,
				Cause:     fmt.Errorf("restart server with sensitive tool permission enabled"),
			}
		}
	}
	return nil
}

type Registry struct {
	mu      sync.RWMutex
	next    uint64
	windows map[string]*WindowTarget
}

type WindowTarget struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"`
	Alias string `json:"alias,omitempty"`
	win   *winput.Window
}

func NewRegistry() *Registry {
	return &Registry{windows: make(map[string]*WindowTarget)}
}

func (r *Registry) AddWindow(win *winput.Window, alias string) *WindowTarget {
	id := fmt.Sprintf("window-%d", atomic.AddUint64(&r.next, 1))
	target := &WindowTarget{ID: id, Kind: "window", Alias: alias, win: win}
	r.mu.Lock()
	r.windows[id] = target
	r.mu.Unlock()
	return target
}

func (r *Registry) Window(id string) (*winput.Window, error) {
	r.mu.RLock()
	target, ok := r.windows[id]
	r.mu.RUnlock()
	if !ok || target.win == nil {
		return nil, targetNotFound(fmt.Errorf("target not found: %s", id))
	}
	return target.win, nil
}

func decodeParams[T any](raw json.RawMessage, out *T) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func decodeArgs[T any](args map[string]any, out *T) error {
	if args == nil {
		args = map[string]any{}
	}
	buf, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, out)
}

func toolResult[T any](data T) ToolCallResult[T] {
	return ToolCallResult[T]{
		StructuredContent: data,
		Content: []ToolContentItem{
			{
				Type: "text",
				Text: marshalTextPayload(data),
			},
		},
		IsError: false,
	}
}

func invalidParams(err error) *JSONRPCErrorObj {
	return &JSONRPCErrorObj{
		Code:    -32602,
		Message: "invalid params",
		Data:    &JSONRPCErrorData{Details: err.Error()},
	}
}

type codedError struct {
	Code      string
	Message   string
	Retryable bool
	Cause     error
}

func (e *codedError) Error() string {
	if e.Cause == nil {
		return e.Message
	}
	return e.Message + ": " + e.Cause.Error()
}

func invalidRequestError(code string, err error) error {
	return &codedError{Code: code, Message: "tool call failed", Retryable: false, Cause: err}
}

func targetNotFound(err error) error {
	return &codedError{Code: "target_not_found", Message: "target not found", Retryable: false, Cause: err}
}

func mapLibraryError(err error) error {
	switch {
	case errors.Is(err, winput.ErrWindowNotFound), errors.Is(err, winput.ErrWindowGone):
		return &codedError{Code: "target_not_found", Message: "target not found", Retryable: false, Cause: err}
	case errors.Is(err, winput.ErrPermissionDenied):
		return &codedError{Code: "permission_denied", Message: "permission denied", Retryable: false, Cause: err}
	case errors.Is(err, winput.ErrDriverNotInstalled), errors.Is(err, winput.ErrDLLLoadFailed), errors.Is(err, winput.ErrDirectedScrollUnsupported):
		return &codedError{Code: "backend_unavailable", Message: "backend unavailable", Retryable: false, Cause: err}
	case errors.Is(err, winput.ErrInvalidScrollSteps), errors.Is(err, winput.ErrInvalidScrollDelta), errors.Is(err, winput.ErrInvalidMouseCoordinate):
		return &codedError{Code: "validation_error", Message: "tool call failed", Retryable: false, Cause: err}
	default:
		return &codedError{Code: "internal_error", Message: "tool call failed", Retryable: false, Cause: err}
	}
}

func toJSONRPCError(err error) *JSONRPCErrorObj {
	var coded *codedError
	if errors.As(err, &coded) {
		return &JSONRPCErrorObj{
			Code:    -32000,
			Message: coded.Message,
			Data: &JSONRPCErrorData{
				Code:      coded.Code,
				Retryable: coded.Retryable,
				Details:   errorDetails(coded.Cause),
			},
		}
	}
	return &JSONRPCErrorObj{Code: -32603, Message: err.Error()}
}

func errorDetails(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// maxRequestBytes is the hard cap for one framed MCP message. The check
// runs before the body is allocated.
const maxRequestBytes = 16 << 20

func readRequest(reader *bufio.Reader) (*JSONRPCRequest, error) {
	firstLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	firstLine = strings.TrimSpace(firstLine)
	if firstLine == "" {
		return readRequest(reader)
	}

	var payload []byte
	if strings.HasPrefix(firstLine, "{") {
		payload = []byte(firstLine)
	} else {
		headers := textproto.MIMEHeader{}
		if err := parseHeaders(reader, headers, firstLine); err != nil {
			return nil, err
		}
		lengthHeader := headers.Get("Content-Length")
		var length int
		if _, err := fmt.Sscanf(lengthHeader, "%d", &length); err != nil || length <= 0 || length > maxRequestBytes {
			return nil, fmt.Errorf("invalid Content-Length header")
		}
		payload = make([]byte, length)
		if _, err := io.ReadFull(reader, payload); err != nil {
			return nil, err
		}
	}

	var req JSONRPCRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, err
	}
	return &req, nil
}

func parseHeaders(reader *bufio.Reader, headers textproto.MIMEHeader, firstLine string) error {
	line := firstLine
	for {
		if line == "" {
			return nil
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid header line: %s", line)
		}
		headers.Add(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))

		next, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(next)
	}
}

func writeResponse(out io.Writer, resp JSONRPCResponse) error {
	payload, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err = out.Write(payload)
	return err
}

// Finder seams keep findWindow's Win32 branches unit-testable without a live
// desktop. They default to the public winput functions; tests swap them out
// and restore via defer.
var (
	findTitle  = winput.FindByTitle
	findClass  = winput.FindByClass
	findByName = winput.FindByProcessName
	findByPID  = winput.FindByPID
)

// findWindow resolves the winning selector (title > class > process_name >
// pid) to a single window. It returns the selected window and, for
// list-based searches (pid/process_name), the candidate count (count is 0
// for title/class single finds, which only accept index 0).
func findWindow(args FindWindowArgs) (*winput.Window, int, error) {
	switch {
	case args.Title != "":
		win, err := findTitle(args.Title)
		if err != nil {
			return nil, 0, targetNotFound(err)
		}
		if args.Index != 0 {
			return nil, 0, invalidRequestError("validation_error", fmt.Errorf("index %d not allowed for title search; only 0 is valid", args.Index))
		}
		return win, 0, nil
	case args.Class != "":
		win, err := findClass(args.Class)
		if err != nil {
			return nil, 0, targetNotFound(err)
		}
		if args.Index != 0 {
			return nil, 0, invalidRequestError("validation_error", fmt.Errorf("index %d not allowed for class search; only 0 is valid", args.Index))
		}
		return win, 0, nil
	case args.ProcessName != "":
		wins, err := findByName(args.ProcessName)
		if err != nil || len(wins) == 0 {
			if err == nil {
				err = winput.ErrWindowNotFound
			}
			return nil, 0, targetNotFound(err)
		}
		win, err := pickCandidate(wins, args.Index)
		if err != nil {
			return nil, 0, invalidRequestError("validation_error", err)
		}
		return win, len(wins), nil
	case args.PID > 0:
		wins, err := findByPID(args.PID)
		if err != nil || len(wins) == 0 {
			if err == nil {
				err = winput.ErrWindowNotFound
			}
			return nil, 0, targetNotFound(err)
		}
		win, err := pickCandidate(wins, args.Index)
		if err != nil {
			return nil, 0, invalidRequestError("validation_error", err)
		}
		return win, len(wins), nil
	default:
		return nil, 0, invalidRequestError("validation_error", fmt.Errorf("one of title, class, process_name, or pid is required"))
	}
}

// pickCandidate selects list[index]; negative or out-of-range indexes are
// rejected (Q4/Q11).
func pickCandidate(list []*winput.Window, index int32) (*winput.Window, error) {
	if index < 0 || int(index) >= len(list) {
		return nil, fmt.Errorf("index %d out of range (candidate count %d)", index, len(list))
	}
	return list[index], nil
}

// requireArgs enforces presence of required arguments at call time (Q13).
// Zero values such as (0, 0) or delta 0 are legal; only an absent key is
// rejected.
func requireArgs(args map[string]any, keys ...string) error {
	for _, k := range keys {
		if _, ok := args[k]; !ok {
			return fmt.Errorf("missing required argument: %s", k)
		}
	}
	return nil
}

// insideVirtualBoundsRule reports whether (x, y) lies inside the half-open
// virtual desktop rectangle [Left, Right) x [Top, Bottom), matching the
// library's hid.insideVirtualBounds rule (Q9).
func insideVirtualBoundsRule(x, y int32, b screen.Rect) bool {
	return x >= b.Left && x < b.Right && y >= b.Top && y < b.Bottom
}

func checkVirtualPoint(x, y int32) error {
	if !insideVirtualBoundsRule(x, y, screen.VirtualBounds()) {
		return fmt.Errorf("point (%d, %d) outside virtual desktop", x, y)
	}
	return nil
}

type clickAtFn func(x, y int32) error

var clickAtFuncs = map[string]clickAtFn{
	"":       winput.ClickMouseAt,
	"left":   winput.ClickMouseAt,
	"right":  winput.ClickRightMouseAt,
	"middle": winput.ClickMiddleMouseAt,
}

func resolveClickAt(button string) (clickAtFn, error) {
	fn, ok := clickAtFuncs[button]
	if !ok {
		return nil, fmt.Errorf("unsupported button: %s", button)
	}
	return fn, nil
}

// keyNames maps the single canonical short name (Q10) for every exported
// Key constant. Published names (ENTER, TAB, ESC, SPACE, CTRL, SHIFT, ALT,
// A-Z, UP, DOWN, LEFT, RIGHT) are unchanged and no second alias set exists.
var keyNames = map[string]winput.Key{
	"A": winput.KeyA, "B": winput.KeyB, "C": winput.KeyC,
	"D": winput.KeyD, "E": winput.KeyE, "F": winput.KeyF,
	"G": winput.KeyG, "H": winput.KeyH, "I": winput.KeyI,
	"J": winput.KeyJ, "K": winput.KeyK, "L": winput.KeyL,
	"M": winput.KeyM, "N": winput.KeyN, "O": winput.KeyO,
	"P": winput.KeyP, "Q": winput.KeyQ, "R": winput.KeyR,
	"S": winput.KeyS, "T": winput.KeyT, "U": winput.KeyU,
	"V": winput.KeyV, "W": winput.KeyW, "X": winput.KeyX,
	"Y": winput.KeyY, "Z": winput.KeyZ,
	"0": winput.Key0, "1": winput.Key1, "2": winput.Key2,
	"3": winput.Key3, "4": winput.Key4, "5": winput.Key5,
	"6": winput.Key6, "7": winput.Key7, "8": winput.Key8,
	"9":   winput.Key9,
	"ESC": winput.KeyEsc,
	"F1":  winput.KeyF1, "F2": winput.KeyF2, "F3": winput.KeyF3,
	"F4": winput.KeyF4, "F5": winput.KeyF5, "F6": winput.KeyF6,
	"F7": winput.KeyF7, "F8": winput.KeyF8, "F9": winput.KeyF9,
	"F10": winput.KeyF10, "F11": winput.KeyF11, "F12": winput.KeyF12,
	"ENTER": winput.KeyEnter, "TAB": winput.KeyTab, "SPACE": winput.KeySpace,
	"BKSP": winput.KeyBkSp,
	"CAPS": winput.KeyCaps, "NUMLOCK": winput.KeyNumLock, "SCROLLLOCK": winput.KeyScroll,
	"CTRL": winput.KeyCtrl, "SHIFT": winput.KeyShift, "ALT": winput.KeyAlt,
	"RIGHTCTRL": winput.KeyRightCtrl, "RIGHTSHIFT": winput.KeyRightShift, "RIGHTALT": winput.KeyRightAlt,
	"LWIN": winput.KeyLWin, "RWIN": winput.KeyRWin,
	"UP": winput.KeyArrowUp, "DOWN": winput.KeyArrowDown, "LEFT": winput.KeyLeft, "RIGHT": winput.KeyRight,
	"HOME": winput.KeyHome, "END": winput.KeyEnd, "INSERT": winput.KeyInsert, "DELETE": winput.KeyDelete,
	"PAGEUP": winput.KeyPageUp, "PAGEDOWN": winput.KeyPageDown,
	"MINUS": winput.KeyMinus, "EQUAL": winput.KeyEqual, "COMMA": winput.KeyComma, "DOT": winput.KeyDot,
	"SLASH": winput.KeySlash, "SEMI": winput.KeySemi, "QUOT": winput.KeyQuot, "TICK": winput.KeyTick,
	"LBR": winput.KeyLBr, "RBR": winput.KeyRBr, "BACKSLASH": winput.KeyBackslash,
	"NUMPAD0": winput.KeyNumpad0, "NUMPAD1": winput.KeyNumpad1, "NUMPAD2": winput.KeyNumpad2,
	"NUMPAD3": winput.KeyNumpad3, "NUMPAD4": winput.KeyNumpad4, "NUMPAD5": winput.KeyNumpad5,
	"NUMPAD6": winput.KeyNumpad6, "NUMPAD7": winput.KeyNumpad7, "NUMPAD8": winput.KeyNumpad8,
	"NUMPAD9": winput.KeyNumpad9, "NUMPADDECIMAL": winput.KeyNumpadDecimal,
	"NUMPADMULTIPLY": winput.KeyNumpadMultiply, "NUMPADADD": winput.KeyNumpadAdd,
	"NUMPADSUBTRACT": winput.KeyNumpadSubtract, "NUMPADDIVIDE": winput.KeyNumpadDivide,
	"NUMPADENTER": winput.KeyNumpadEnter,
}

// maxPressHotkeyKeys caps the keys array on press_hotkey (ENGINEERING §7;
// enforced at call time per Q13).
const maxPressHotkeyKeys = 8

// keyFromName resolves a key short name or a single non-shift character
// (Q6/Q10): trim whitespace, match a case-insensitive short name, then fall
// back to keyboard.LookupKey for a single character that needs no shift.
// Shift-requiring characters are rejected with a hint to use press_hotkey.
func keyFromName(name string) (winput.Key, error) {
	trimmed := strings.TrimSpace(name)
	if k, ok := keyNames[strings.ToUpper(trimmed)]; ok {
		return k, nil
	}
	runes := []rune(trimmed)
	if len(runes) == 1 {
		if k, shifted, ok := keyboard.LookupKey(runes[0]); ok {
			if shifted {
				return 0, fmt.Errorf("character %q requires shift; use a hotkey such as [\"SHIFT\",\"1\"]", runes[0])
			}
			return k, nil
		}
	}
	return 0, fmt.Errorf("unsupported key name: %s", name)
}

func describeWindowTarget(target *WindowTarget, win *winput.Window) WindowTargetResult {
	result := WindowTargetResult{
		TargetID:  target.ID,
		Kind:      target.Kind,
		IsValid:   false,
		IsVisible: false,
	}
	if win == nil {
		return result
	}

	result.IsValid = win.IsValid()
	result.IsVisible = win.IsVisible()

	if width, height, err := win.ClientRect(); err == nil {
		result.ClientWidth = width
		result.ClientHeight = height
	}
	if dpiX, dpiY, err := win.DPI(); err == nil {
		result.DPIX = dpiX
		result.DPIY = dpiY
	}
	if target.Alias != "" {
		result.Alias = target.Alias
	}
	return result
}
