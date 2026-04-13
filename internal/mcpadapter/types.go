package mcpadapter

import "encoding/json"

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
	Capabilities    map[string]any         `json:"capabilities"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type ToolListResult struct {
	Tools []Tool `json:"tools"`
}

type JSONRPCErrorData struct {
	Code      string `json:"code"`
	Retryable bool   `json:"retryable"`
	Details   string `json:"details,omitempty"`
}

type JSONRPCErrorObj struct {
	Code    int               `json:"code"`
	Message string            `json:"message"`
	Data    *JSONRPCErrorData `json:"data,omitempty"`
}

type ToolContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type ToolCallResult[T any] struct {
	StructuredContent T                 `json:"structuredContent"`
	Content           []ToolContentItem `json:"content"`
	IsError           bool              `json:"isError"`
}

type WindowTargetResult struct {
	TargetID       string `json:"target_id"`
	Kind           string `json:"kind"`
	Alias          string `json:"alias,omitempty"`
	Title          string `json:"title,omitempty"`
	Class          string `json:"class,omitempty"`
	ProcessName    string `json:"process_name,omitempty"`
	ParentTargetID string `json:"parent_target_id,omitempty"`
	IsValid        bool   `json:"is_valid"`
	IsVisible      bool   `json:"is_visible"`
	ClientWidth    int32  `json:"client_width,omitempty"`
	ClientHeight   int32  `json:"client_height,omitempty"`
	DPIX           uint32 `json:"dpi_x,omitempty"`
	DPIY           uint32 `json:"dpi_y,omitempty"`
}

type OKResult struct {
	OK bool `json:"ok"`
}

type SwitchBackendResult struct {
	OK      bool   `json:"ok"`
	Backend string `json:"backend"`
}

type ReadTextResult struct {
	Text string `json:"text"`
}

type CaptureScreenResult struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

type CursorPosResult struct {
	X int32 `json:"x"`
	Y int32 `json:"y"`
}

type VirtualBoundsResult struct {
	Left   int32 `json:"left"`
	Top    int32 `json:"top"`
	Right  int32 `json:"right"`
	Bottom int32 `json:"bottom"`
}

func marshalTextPayload(v any) string {
	buf, _ := json.Marshal(v)
	return string(buf)
}
