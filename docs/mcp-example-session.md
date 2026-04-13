# MCP Example Session

This document shows a minimal stdio session against `cmd/mcp-server`.

## Start the Server

Read-only tools only:

```bash
go run ./cmd/mcp-server
```

Enable mutating tools:

```bash
go run ./cmd/mcp-server -allow-mutations
```

Enable mutating and sensitive tools:

```bash
go run ./cmd/mcp-server -allow-mutations -allow-sensitive
```

## Initialize

Request:

```text
Content-Length: 58

{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}
```

Response shape:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2025-04-01",
    "serverInfo": {
      "name": "winput-mcp",
      "version": "0.1.0"
    },
    "capabilities": {
      "tools": {}
    }
  }
}
```

## List Tools

Request:

```text
Content-Length: 58

{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}
```

Response highlights:
- each tool includes `inputSchema`
- each tool includes `outputSchema`
- each tool includes `errorSchema`
- annotations include `sideEffectLevel` and `requiresConfirmation`
- `find_window` and `find_child` expose runtime metadata such as visibility, client size, and DPI when available

## Call a Read-Only Tool

Request:

```text
Content-Length: 66

{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{
  "name":"get_virtual_bounds",
  "arguments":{}
}}
```

Successful response shape:

```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "result": {
    "structuredContent": {
      "left": 0,
      "top": 0,
      "right": 1920,
      "bottom": 1080
    },
    "content": [
      {
        "type": "text",
        "text": "{\"left\":0,\"top\":0,\"right\":1920,\"bottom\":1080}"
      }
    ],
    "isError": false
  }
}
```

## Call a Mutating Tool Without Permission

Request:

```text
Content-Length: 126

{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{
  "name":"click",
  "arguments":{"target_id":"window-1","x":100,"y":100}
}}
```

Error response shape:

```json
{
  "jsonrpc": "2.0",
  "id": 4,
  "error": {
    "code": -32000,
    "message": "mutating tools are disabled",
    "data": {
      "code": "unsafe_operation",
      "retryable": false,
      "details": "restart server with mutation permission enabled"
    }
  }
}
```

## Target Flow

Typical tool sequence:
1. `find_window`
2. `find_child` if needed
3. reuse returned `target_id` in `click`, `type_text`, `read_text`, or key tools

The server intentionally exposes `target_id` instead of raw HWND values so clients can stay decoupled from Windows internals.

## Richer `find_window` Result

A successful `find_window` call now returns more than just the opaque identifier. Typical fields include:

```json
{
  "target_id": "window-1",
  "kind": "window",
  "alias": "Untitled - Notepad",
  "title": "Untitled - Notepad",
  "class": "",
  "process_name": "",
  "is_valid": true,
  "is_visible": true,
  "client_width": 1264,
  "client_height": 681,
  "dpi_x": 96,
  "dpi_y": 96
}
```

These fields are intended to help external agents reason about target quality and layout without exposing raw Win32 handles.
