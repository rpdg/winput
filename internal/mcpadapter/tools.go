package mcpadapter

type Tool struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	OutputSchema map[string]any `json:"outputSchema,omitempty"`
	ErrorSchema map[string]any `json:"errorSchema,omitempty"`
	Annotations map[string]any `json:"annotations,omitempty"`
}

type ToolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type FindWindowArgs struct {
	Title       string `json:"title,omitempty"`
	Class       string `json:"class,omitempty"`
	ProcessName string `json:"process_name,omitempty"`
	// PID 0 means "not provided" (Q12); legal values are 1..math.MaxUint32.
	PID uint32 `json:"pid,omitempty"`
	// Index selects a window from the pid/process_name candidate list
	// (default 0). For title/class searches only 0 is valid (Q11).
	Index int32 `json:"index,omitempty"`
}

func (a FindWindowArgs) alias() string {
	switch {
	case a.Title != "":
		return a.Title
	case a.Class != "":
		return a.Class
	default:
		return a.ProcessName
	}
}

type FindChildArgs struct {
	ParentTargetID string `json:"parent_target_id"`
	Class          string `json:"class"`
}

// ScreenPointArgs carries absolute virtual-desktop coordinates for the
// screen-coordinate input tools. Coordinates are validated for presence
// (Q13) because (0,0) is itself a legal point.
type ScreenPointArgs struct {
	X int32 `json:"x"`
	Y int32 `json:"y"`
}

type ClickAtArgs struct {
	X      int32  `json:"x"`
	Y      int32  `json:"y"`
	Button string `json:"button,omitempty"`
}

// ScrollArgs is the window-directed wheel tool. delta uses Win32 wheel
// increments (120 per notch); an explicit 0 is a successful no-op (Q8/Q13).
type ScrollArgs struct {
	TargetID string `json:"target_id"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
	Delta    int32  `json:"delta"`
}

type ScrollWheelArgs struct {
	Steps int `json:"steps"`
}

type ClickArgs struct {
	TargetID string `json:"target_id"`
	X        int32  `json:"x"`
	Y        int32  `json:"y"`
	Button   string `json:"button,omitempty"`
}

type TypeTextArgs struct {
	TargetID string `json:"target_id"`
	Text     string `json:"text"`
}

type PressKeyArgs struct {
	TargetID string `json:"target_id"`
	Key      string `json:"key"`
}

type PressHotkeyArgs struct {
	TargetID string   `json:"target_id"`
	Keys     []string `json:"keys"`
}

type ReadTextArgs struct {
	TargetID   string `json:"target_id"`
	BestEffort bool   `json:"best_effort,omitempty"`
}

type SwitchBackendArgs struct {
	Backend string `json:"backend"`
}

// keyNamesDescription is the discovery surface for the key short-name
// contract (Q10): it must enumerate the full canonical table and never
// advertise rejected aliases (BACKSPACE, ESCAPE, LEFTCTRL, ARROWUP,
// DIVIDE).
const keyNamesDescription = "Key names: A-Z, 0-9; ESC, F1-F12, ENTER, TAB, SPACE, BKSP; " +
	"CAPS, NUMLOCK, SCROLLLOCK; CTRL, SHIFT, ALT (left), RIGHTCTRL, RIGHTSHIFT, RIGHTALT; " +
	"LWIN, RWIN; UP, DOWN, LEFT, RIGHT; HOME, END, INSERT, DELETE, PAGEUP, PAGEDOWN; " +
	"MINUS, EQUAL, COMMA, DOT, SLASH, SEMI, QUOT, TICK, LBR, RBR, BACKSLASH; " +
	"NUMPAD0-NUMPAD9, NUMPADDECIMAL, NUMPADMULTIPLY, NUMPADADD, NUMPADSUBTRACT, NUMPADDIVIDE, NUMPADENTER. " +
	"A single character that needs no shift (such as , . - ; [ ] \\ ' ` = /) is also accepted; " +
	"a character that needs shift must use a hotkey, for example [\"SHIFT\",\"1\"]."

func defaultCatalog() []Tool {
	commonErrorSchema := objectSchema(map[string]any{
		"code":      requiredStringSchema("Stable machine-readable error code."),
		"message":   requiredStringSchema("Human-readable error summary."),
		"retryable": boolSchema("Whether the caller may retry safely."),
		"details":   stringSchema("Optional low-level detail."),
	}, "code", "message", "retryable")

	return []Tool{
		{
			Name:  "find_window",
			Title: "Find Window",
			Description: "Find a top-level window by title, class, process name, or PID and return an opaque target_id. " +
				"Selector precedence: title > class > process_name > pid; extra non-empty selectors are ignored. " +
				"For process_name or pid the hit is chosen from a candidate list via index (default 0) and count reports how many windows matched. " +
				"PID candidates come from the current EnumWindows Z-order and may include hidden top-level windows; " +
				"process_name collects top-level windows from every process with that executable name, in Toolhelp snapshot order. " +
				"The returned title, class and process_name fields echo the requested filters, not the window's own identity. " +
				"To learn a window's actual identity, bind the returned target_id and call read_text.",
			InputSchema: objectSchema(map[string]any{
				"title":        stringSchema("Exact window title."),
				"class":        stringSchema("Window class name."),
				"process_name": stringSchema("Process executable name, for example notepad.exe."),
				"pid":          intSchema("Process ID. 0 or omitted means unused; candidates are enumerated in the current Z-order."),
				"index":        intSchema("Index into the candidate list for process_name/pid searches, default 0; for title/class only 0 is valid."),
			}),
			OutputSchema: objectSchema(map[string]any{
				"target_id":     requiredStringSchema("Opaque target identifier for later tool calls."),
				"kind":          requiredStringSchema("Target kind."),
				"alias":         stringSchema("Server-side alias derived from the lookup input."),
				"title":         stringSchema("Requested title filter when provided."),
				"class":         stringSchema("Requested class filter when provided."),
				"process_name":  stringSchema("Requested process name filter when provided."),
				"count":         intSchema("Number of matching candidates for process_name/pid searches; omitted otherwise."),
				"is_valid":      boolSchema("Whether the target handle is currently valid."),
				"is_visible":    boolSchema("Whether the target window is visible and not minimized."),
				"client_width":  intSchema("Client area width when available."),
				"client_height": intSchema("Client area height when available."),
				"dpi_x":         intSchema("Window horizontal DPI when available."),
				"dpi_y":         intSchema("Window vertical DPI when available."),
			}, "target_id", "kind", "is_valid", "is_visible"),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("read_only", false),
		},
		{
			Name:        "find_child",
			Title:       "Find Child",
			Description: "Find a child window by class under an existing target_id.",
			InputSchema: objectSchema(map[string]any{
				"parent_target_id": requiredStringSchema("Parent target_id."),
				"class":            requiredStringSchema("Child window class."),
			}, "parent_target_id", "class"),
			OutputSchema: objectSchema(map[string]any{
				"target_id":        requiredStringSchema("Opaque child target identifier."),
				"kind":             requiredStringSchema("Target kind."),
				"alias":            stringSchema("Server-side alias derived from the lookup input."),
				"class":            requiredStringSchema("Requested child class."),
				"parent_target_id": requiredStringSchema("Parent target identifier used for the lookup."),
				"is_valid":         boolSchema("Whether the target handle is currently valid."),
				"is_visible":       boolSchema("Whether the child target is visible and not minimized."),
				"client_width":     intSchema("Client area width when available."),
				"client_height":    intSchema("Client area height when available."),
				"dpi_x":            intSchema("Window horizontal DPI when available."),
				"dpi_y":            intSchema("Window vertical DPI when available."),
			}, "target_id", "kind", "class", "parent_target_id", "is_valid", "is_visible"),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("read_only", false),
		},
		{
			Name:        "click",
			Title:       "Click",
			Description: "Click at client coordinates inside a target window.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id returned by find_window or find_child."),
				"x":         intSchema("Client X coordinate."),
				"y":         intSchema("Client Y coordinate."),
				"button":    stringEnumSchema("Mouse button.", "left", "right", "middle"),
			}, "target_id", "x", "y"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "double_click",
			Title:       "Double Click",
			Description: "Double-click at client coordinates inside a target window.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"x":         intSchema("Client X coordinate."),
				"y":         intSchema("Client Y coordinate."),
			}, "target_id", "x", "y"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "right_click",
			Title:       "Right Click",
			Description: "Right-click at client coordinates inside a target window.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"x":         intSchema("Client X coordinate."),
				"y":         intSchema("Client Y coordinate."),
			}, "target_id", "x", "y"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "move_mouse",
			Title:       "Move Mouse",
			Description: "Move the mouse to client coordinates inside a target window.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"x":         intSchema("Client X coordinate."),
				"y":         intSchema("Client Y coordinate."),
			}, "target_id", "x", "y"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "type_text",
			Title:       "Type Text",
			Description: "Type text into a target window or control.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"text":      requiredStringSchema("Text to type."),
			}, "target_id", "text"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "press_key",
			Title:       "Press Key",
			Description: "Press a single key on a target window. " + keyNamesDescription,
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"key":       requiredStringSchema(keyNamesDescription),
			}, "target_id", "key"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "press_hotkey",
			Title:       "Press Hotkey",
			Description: "Press a key combination on a target window (max 8 keys). " + keyNamesDescription,
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"keys": map[string]any{
					"type":        "array",
					"description": "Ordered key names; see the key-name list in the description above. Max 8.",
					"items": map[string]any{
						"type": "string",
					},
					"minItems": 1,
					"maxItems": 8,
				},
			}, "target_id", "keys"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "read_text",
			Title:       "Read Text",
			Description: "Read text from a target window or control.",
			InputSchema: objectSchema(map[string]any{
				"target_id":   requiredStringSchema("Opaque target_id."),
				"best_effort": boolSchema("Use UI Automation fallback when available."),
			}, "target_id"),
			OutputSchema: objectSchema(map[string]any{
				"text": requiredStringSchema("Text returned from the target."),
			}, "text"),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("read_only", false),
		},
		{
			Name:        "capture_screen",
			Title:       "Capture Screen",
			Description: "Capture the virtual desktop and return basic dimensions.",
			InputSchema: objectSchema(map[string]any{}),
			OutputSchema: objectSchema(map[string]any{
				"width":  intSchema("Captured image width in pixels."),
				"height": intSchema("Captured image height in pixels."),
			}, "width", "height"),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("read_only", false),
		},
		{
			Name:        "switch_backend",
			Title:       "Switch Backend",
			Description: "Switch the active input backend to message or hid.",
			InputSchema: objectSchema(map[string]any{
				"backend": stringEnumSchema("Target backend.", "message", "hid"),
			}, "backend"),
			OutputSchema: objectSchema(map[string]any{
				"ok":      boolSchema("Whether the backend switch succeeded."),
				"backend": requiredStringSchema("The active backend after the switch."),
			}, "ok", "backend"),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("sensitive", true),
		},
		{
			Name:        "get_cursor_pos",
			Title:       "Get Cursor Position",
			Description: "Get the current absolute cursor coordinates.",
			InputSchema: objectSchema(map[string]any{}),
			OutputSchema: objectSchema(map[string]any{
				"x": intSchema("Current cursor X coordinate."),
				"y": intSchema("Current cursor Y coordinate."),
			}, "x", "y"),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("read_only", false),
		},
		{
			Name:        "get_virtual_bounds",
			Title:       "Get Virtual Bounds",
			Description: "Get the virtual desktop bounds across all monitors.",
			InputSchema: objectSchema(map[string]any{}),
			OutputSchema: objectSchema(map[string]any{
				"left":   intSchema("Virtual desktop left coordinate."),
				"top":    intSchema("Virtual desktop top coordinate."),
				"right":  intSchema("Virtual desktop right coordinate."),
				"bottom": intSchema("Virtual desktop bottom coordinate."),
			}, "left", "top", "right", "bottom"),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("read_only", false),
		},
		{
			Name:        "move_mouse_to",
			Title:       "Move Mouse To",
			Description: "Move the mouse cursor to absolute virtual-desktop coordinates.",
			InputSchema: objectSchema(map[string]any{
				"x": intSchema("Virtual desktop X coordinate."),
				"y": intSchema("Virtual desktop Y coordinate."),
			}, "x", "y"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "click_at",
			Title:       "Click At",
			Description: "Move the mouse to absolute virtual-desktop coordinates and click.",
			InputSchema: objectSchema(map[string]any{
				"x":      intSchema("Virtual desktop X coordinate."),
				"y":      intSchema("Virtual desktop Y coordinate."),
				"button": stringEnumSchema("Mouse button.", "left", "right", "middle"),
			}, "x", "y"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "double_click_at",
			Title:       "Double Click At",
			Description: "Move the mouse to absolute virtual-desktop coordinates and double-click.",
			InputSchema: objectSchema(map[string]any{
				"x": intSchema("Virtual desktop X coordinate."),
				"y": intSchema("Virtual desktop Y coordinate."),
			}, "x", "y"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "scroll_wheel",
			Title:       "Scroll Wheel",
			Description: "Scroll by whole wheel steps. Applies to whatever window is under the current cursor; position the cursor first with move_mouse_to.",
			InputSchema: objectSchema(map[string]any{
				"steps": intSchema("Whole wheel steps; positive scrolls forward."),
			}, "steps"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "scroll",
			Title:       "Scroll",
			Description: "Send a wheel event at client coordinates inside a target window. delta uses Win32 wheel increments (120 per notch) with the same direction as scroll_wheel steps; an explicit 0 is a successful no-op. Works on the message backend; the HID backend returns backend_unavailable because hardware wheel events cannot be directed to a window.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"x":         intSchema("Client X coordinate."),
				"y":         intSchema("Client Y coordinate."),
				"delta":     intSchema("Wheel delta in Win32 increments (120 per notch); explicit 0 is a no-op."),
			}, "target_id", "x", "y", "delta"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
	}
}

func annotations(sideEffect string, confirm bool) map[string]any {
	return map[string]any{
		"sideEffectLevel":      sideEffect,
		"requiresConfirmation": confirm,
	}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func stringSchema(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func requiredStringSchema(desc string) map[string]any {
	return stringSchema(desc)
}

func intSchema(desc string) map[string]any {
	return map[string]any{"type": "integer", "description": desc}
}

func boolSchema(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func stringEnumSchema(desc string, values ...string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": desc,
		"enum":        values,
	}
}

func okSchema() map[string]any {
	return objectSchema(map[string]any{
		"ok": boolSchema("Whether the operation succeeded."),
	}, "ok")
}
