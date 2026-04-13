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
	catalog         []Tool
	registry        *Registry
	allowMutations  bool
	allowSensitive  bool
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
		resp.Result = InitializeResult{
			ProtocolVersion: "2025-04-01",
			ServerInfo: ServerInfo{
				Name:    "winput-mcp",
				Version: "0.1.0",
			},
			Capabilities: map[string]any{
				"tools": map[string]any{},
			},
		}
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
		win, err := findWindow(args)
		if err != nil {
			return nil, err
		}
		target := s.registry.AddWindow(win, args.alias())
		result := describeWindowTarget(target, win)
		result.Title = args.Title
		result.Class = args.Class
		result.ProcessName = args.ProcessName
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
	case errors.Is(err, winput.ErrDriverNotInstalled), errors.Is(err, winput.ErrDLLLoadFailed):
		return &codedError{Code: "backend_unavailable", Message: "backend unavailable", Retryable: false, Cause: err}
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
		if _, err := fmt.Sscanf(lengthHeader, "%d", &length); err != nil || length <= 0 {
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

func findWindow(args FindWindowArgs) (*winput.Window, error) {
	switch {
	case args.Title != "":
		win, err := winput.FindByTitle(args.Title)
		if err != nil {
			return nil, targetNotFound(err)
		}
		return win, nil
	case args.Class != "":
		win, err := winput.FindByClass(args.Class)
		if err != nil {
			return nil, targetNotFound(err)
		}
		return win, nil
	case args.ProcessName != "":
		wins, err := winput.FindByProcessName(args.ProcessName)
		if err != nil || len(wins) == 0 {
			if err == nil {
				err = winput.ErrWindowNotFound
			}
			return nil, targetNotFound(err)
		}
		return wins[0], nil
	default:
		return nil, invalidRequestError("validation_error", fmt.Errorf("one of title, class, or process_name is required"))
	}
}

func keyFromName(name string) (winput.Key, error) {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "ENTER":
		return winput.KeyEnter, nil
	case "TAB":
		return winput.KeyTab, nil
	case "ESC":
		return winput.KeyEsc, nil
	case "SPACE":
		return winput.KeySpace, nil
	case "CTRL":
		return winput.KeyCtrl, nil
	case "SHIFT":
		return winput.KeyShift, nil
	case "ALT":
		return winput.KeyAlt, nil
	case "A":
		return winput.KeyA, nil
	case "B":
		return winput.KeyB, nil
	case "C":
		return winput.KeyC, nil
	case "D":
		return winput.KeyD, nil
	case "E":
		return winput.KeyE, nil
	case "F":
		return winput.KeyF, nil
	case "G":
		return winput.KeyG, nil
	case "H":
		return winput.KeyH, nil
	case "I":
		return winput.KeyI, nil
	case "J":
		return winput.KeyJ, nil
	case "K":
		return winput.KeyK, nil
	case "L":
		return winput.KeyL, nil
	case "M":
		return winput.KeyM, nil
	case "N":
		return winput.KeyN, nil
	case "O":
		return winput.KeyO, nil
	case "P":
		return winput.KeyP, nil
	case "Q":
		return winput.KeyQ, nil
	case "R":
		return winput.KeyR, nil
	case "S":
		return winput.KeyS, nil
	case "T":
		return winput.KeyT, nil
	case "U":
		return winput.KeyU, nil
	case "V":
		return winput.KeyV, nil
	case "W":
		return winput.KeyW, nil
	case "X":
		return winput.KeyX, nil
	case "Y":
		return winput.KeyY, nil
	case "Z":
		return winput.KeyZ, nil
	case "UP":
		return winput.KeyArrowUp, nil
	case "DOWN":
		return winput.KeyArrowDown, nil
	case "LEFT":
		return winput.KeyLeft, nil
	case "RIGHT":
		return winput.KeyRight, nil
	default:
		return 0, fmt.Errorf("unsupported key name: %s", name)
	}
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
