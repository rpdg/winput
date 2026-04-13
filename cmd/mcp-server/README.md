# MCP Server

This directory contains the `winput` MCP server adapter entrypoint.

Its purpose is to expose a stable subset of `winput` capabilities as MCP tools for external AI systems.

## Intended Responsibilities

- publish a tool catalog
- validate tool inputs
- call into `winput`
- normalize outputs and errors
- enforce confirmation for risky tools
- serve a minimal MCP-compatible JSON-RPC interface over stdio

## Explicit Non-Goals

- do not implement agent planning here
- do not define reusable skills here
- do not add OCR, CV, or DOM integrations in the first version
- do not change the public Go library API just to fit the server layer

## Initial Tool Candidates

- `find_window`
- `find_child`
- `click`
- `double_click`
- `right_click`
- `move_mouse`
- `type_text`
- `press_key`
- `press_hotkey`
- `read_text`
- `capture_screen`
- `switch_backend`
- `get_cursor_pos`
- `get_virtual_bounds`

See [docs/mcp-tools.md](/D:/Works/Personal/GoLang/winput/docs/mcp-tools.md) for the design and boundary rules.

## Current Status

The repository now includes a minimal server skeleton in `main.go` and an internal adapter package.
The current implementation focuses on the initial low-level tool catalog and in-memory target registry rather than a full agent runtime.

## Protocol Shape

The server currently exposes:
- `initialize`
- `tools/list`
- `tools/call`

Transport:
- stdio
- JSON-RPC messages
- `Content-Length` framed payloads

`tools/list` returns:
- tool metadata
- `inputSchema`
- `outputSchema`
- `errorSchema`
- side-effect annotations

`tools/call` returns:
- typed `structuredContent`
- text `content`
- stable structured error data on failure

## Safety Defaults

By default, mutating and sensitive tools are blocked.

Use flags to enable them explicitly:

```bash
go run ./cmd/mcp-server -allow-mutations -allow-sensitive
```

Current policy:
- read-only tools are enabled by default
- state-changing tools require `-allow-mutations`
- sensitive tools require `-allow-sensitive`

## Documentation

- adapter design: [docs/mcp-tools.md](/D:/Works/Personal/GoLang/winput/docs/mcp-tools.md)
- example session: [docs/mcp-example-session.md](/D:/Works/Personal/GoLang/winput/docs/mcp-example-session.md)
