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

func defaultCatalog() []Tool {
	commonErrorSchema := objectSchema(map[string]any{
		"code":      requiredStringSchema("Stable machine-readable error code."),
		"message":   requiredStringSchema("Human-readable error summary."),
		"retryable": boolSchema("Whether the caller may retry safely."),
		"details":   stringSchema("Optional low-level detail."),
	}, "code", "message", "retryable")

	return []Tool{
		{
			Name:        "find_window",
			Title:       "Find Window",
			Description: "Find a top-level window by title, class, or process name and return an opaque target_id.",
			InputSchema: objectSchema(map[string]any{
				"title":        stringSchema("Exact window title."),
				"class":        stringSchema("Window class name."),
				"process_name": stringSchema("Process executable name, for example notepad.exe."),
			}),
			OutputSchema: objectSchema(map[string]any{
				"target_id":     requiredStringSchema("Opaque target identifier for later tool calls."),
				"kind":          requiredStringSchema("Target kind."),
				"alias":         stringSchema("Server-side alias derived from the lookup input."),
				"title":         stringSchema("Requested title filter when provided."),
				"class":         stringSchema("Requested class filter when provided."),
				"process_name":  stringSchema("Requested process name filter when provided."),
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
			Description: "Press a single key on a target window.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"key":       requiredStringSchema("Key name such as ENTER, TAB, A, CTRL."),
			}, "target_id", "key"),
			OutputSchema: okSchema(),
			ErrorSchema: commonErrorSchema,
			Annotations: annotations("state_change", true),
		},
		{
			Name:        "press_hotkey",
			Title:       "Press Hotkey",
			Description: "Press a key combination on a target window.",
			InputSchema: objectSchema(map[string]any{
				"target_id": requiredStringSchema("Opaque target_id."),
				"keys": map[string]any{
					"type":        "array",
					"description": "Ordered key names such as CTRL and A.",
					"items": map[string]any{
						"type": "string",
					},
					"minItems": 1,
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
