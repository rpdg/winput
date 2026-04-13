# MCP Tools Adapter Design

## Summary

`winput` should remain a focused Windows input automation library while also exposing its stable low-level capabilities to external AI systems through an MCP server adapter.

The MCP layer belongs in this repository because it is a direct packaging of existing `winput` capabilities. It should not turn the core library into an agent runtime. Planning, skills, OCR, CV, DOM automation, and higher-level recovery logic remain outside the core library surface.

Recommended repository split:
- core library: `winput`, `screen`, `window`, `keyboard`, `mouse`, `hid`, `uia`
- MCP adapter layer: `cmd/mcp-server`

## Goals

- Make `winput` discoverable and callable by external AI agents
- Expose stable, schema-based tools over MCP
- Preserve the current Go API as the primary library contract
- Keep the MCP adapter isolated from the core package design

## Non-Goals

- Do not embed a full agent runtime in this repository
- Do not add skills, planners, or plan execution semantics here
- Do not expose provider-native handles such as HWND values as public MCP contracts
- Do not fold OCR, CV, or DOM logic into the first MCP adapter version

## Layering

### Core Library

The core library remains responsible for:
- window discovery
- child control discovery
- keyboard and mouse input
- Win32/UIA text reading
- screen capture
- DPI and coordinate conversion

### MCP Adapter

The MCP adapter is responsible for:
- tool discovery
- input validation
- structured output normalization
- structured error mapping
- confirmation gates for risky tools
- MCP transport and lifecycle

The adapter must call into `winput` as a consumer. It must not require changes to the public `winput` API just to satisfy MCP protocol shape.

## Initial Tool Set

The first MCP tool catalog should expose only stable, low-level actions:

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

These tools should map directly to existing `winput` capabilities or adjacent package functions already in this repository.

For target-discovery tools such as `find_window` and `find_child`, the adapter should also return stable runtime metadata when available, including:
- visibility
- client area size
- DPI
- requested lookup hints

## Tool Contract Requirements

Each MCP tool should provide:
- stable `name`
- concise `description`
- JSON Schema `input_schema`
- JSON Schema `output_schema`
- structured error codes
- side-effect metadata
- confirmation requirement metadata where needed

Recommended side-effect levels:
- `read_only`
- `state_change`
- `sensitive`
- `destructive`

Recommended error codes:
- `validation_error`
- `permission_denied`
- `target_not_found`
- `timeout`
- `backend_unavailable`
- `unsafe_operation`
- `internal_error`

## Public Boundary Rules

- MCP contracts should use runtime-neutral target descriptions, not provider-native handles
- Tool outputs should be useful to external agents without exposing internals
- The adapter should normalize library errors into machine-actionable MCP responses
- Tool names should stay stable over time
- Breaking schema changes should use a new spec major version or a new tool name

## Suggested Directory Layout

Recommended first layout:

- `cmd/mcp-server`
  - MCP server entrypoint
  - tool registration
  - schema definitions
  - error mapping
  - confirmation policy

- `docs/mcp-tools.md`
  - adapter design and boundary rules

Optional follow-up directories:
- `internal/mcpadapter`
- `internal/toolcatalog`

## Development Sequence

1. Create the MCP server module entrypoint
2. Define the tool catalog and JSON Schemas
3. Implement tool-to-library mappings for the initial tool set
4. Normalize outputs and errors
5. Add confirmation handling for risky tools
6. Add documentation and example client usage

## Current Implemented Pieces

The repository now includes:
- a minimal stdio MCP server entrypoint in `cmd/mcp-server`
- initial tool discovery through `tools/list`
- `tools/call` mappings for the initial low-level tool catalog
- JSON Schema input and output metadata on each tool
- structured error metadata per tool
- in-memory `target_id` registry for window and child-window flows
- safety gates for mutating and sensitive tools
- framed `Content-Length` request and response transport

Example protocol usage is documented in `docs/mcp-example-session.md`.

## Compatibility Strategy

Use independent versioning for:
- the Go library API
- the MCP adapter spec

The MCP adapter should evolve additively where possible:
- add tools without changing existing ones
- add optional fields without removing required ones
- reserve breaking schema changes for major version increments
