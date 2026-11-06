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

## Tool Catalog

The catalog currently exposes 19 stable low-level tools:

- `find_window` (selector precedence: title > class > process_name > pid; list searches report `count` and select via `index`)
- `find_child`
- `click`
- `double_click`
- `right_click`
- `move_mouse`
- `type_text`
- `press_key`
- `press_hotkey` (keys capped at 8)
- `read_text`
- `capture_screen`
- `switch_backend`
- `get_cursor_pos`
- `get_virtual_bounds`
- `move_mouse_to`
- `click_at`
- `double_click_at`
- `scroll_wheel`
- `scroll`

See [docs/mcp-tools-gaps.md](../../docs/mcp-tools-gaps.md) for the gap-fill design and decisions, and [docs/mcp-tools.md](../../docs/mcp-tools.md) for the adapter design and boundary rules.

## Current Status

The repository includes a stdio MCP server entrypoint in `main.go` and the `internal/mcpadapter` package.
The implementation covers the low-level tool catalog, window-target discovery and registry, screen-coordinate input, directed scrolling, structured errors, and safety gates over a hand-rolled JSON-RPC stdio transport.

## Protocol Shape

The server currently exposes:
- `initialize` (with protocol version negotiation)
- `ping`
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

Call-time validation: new tools require their coordinate/delta arguments (an absent argument is a validation error even though the zero value is itself legal); `press_hotkey` rejects more than 8 keys.

## Capture Screen DPI

`capture_screen` requires per-monitor DPI awareness. On startup the server calls `winput.EnablePerMonitorDPI()` and verifies `window.IsPerMonitorDPIAware()`; when awareness cannot be established it logs a warning and keeps running — `capture_screen` then fails with the library's DPI error.

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

- adapter design: [docs/mcp-tools.md](../../docs/mcp-tools.md)
- example session: [docs/mcp-example-session.md](../../docs/mcp-example-session.md)
