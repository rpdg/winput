# MCP Tools Adapter Design

## Summary

`winput` should remain a focused Windows input automation library while also exposing its stable low-level capabilities to external AI systems through an MCP server adapter.

The MCP layer belongs in this repository because it is a direct packaging of existing `winput` capabilities. It should not turn the core library into an agent runtime. Planning, skills, OCR, CV, DOM automation, and higher-level recovery logic remain outside the core library surface.

This file is the single adapter design and decision record. The former separate gap-fill document has been merged into the sections below; decisions Q1–Q13 are recorded in place and summarized in the change log.

Recommended repository split:
- core library: `winput`, `screen`, `window`, `keyboard`, `mouse`, `hid`, `uia`
- MCP adapter layer: `cmd/mcp-server` + `internal/mcpadapter`

## Goals

- Make `winput` discoverable and callable by external AI agents
- Expose stable, schema-based tools over MCP
- Preserve the current Go API as the primary library contract
- Keep the MCP adapter isolated from the core package design

## Non-Goals

- Do not embed a full agent runtime in this repository
- Do not add skills, planners, or plan execution semantics here
- Do not expose provider-native handles such as HWND values as public MCP contracts
- Do not fold OCR, CV, or DOM logic into the MCP adapter
- Do not add MCP-specific interfaces to the core library
- Do not implement screen discovery or image return in this phase. `capture_screen` reports dimensions only; the caller already holds the virtual-desktop coordinates for the input tools.

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

### Layering Rules

- The adapter is a consumer of existing `winput`/`screen` exports; no new tool may change the core public API.
- New tools evolve additively: no required field is added to an existing tool, and no existing field type is changed.
- `target_id` is opaque and valid only inside one server process; it is not promised across processes or restarts.
- Provider-native handles (HWND, driver handles) never enter the MCP contract.

## Capability Matrix

Coverage of the public API as of this design:

| Public API | MCP tool | Status |
|---|---|---|
| `FindByTitle` / `FindByClass` / `FindByProcessName` | `find_window` | Covered; `process_name` candidates use `index` and report `count` |
| `FindByPID` | `find_window` (`pid`) | Covered |
| `FindChildByClass` | `find_child` | Covered |
| `Window.Move` / `Click` / `Right` / `Middle` / `DoubleClick` | `move_mouse` / `click` / `right_click` / `double_click` | Covered |
| `Window.Type` / `Press` / `PressHotkey` | `type_text` / `press_key` / `press_hotkey` | Covered; key short-name table is complete |
| `Window.Text` / `Value` | `read_text` (`best_effort`) | Covered |
| `Window.Scroll` | `scroll` | Covered |
| `Window.MoveRel` | — | Later |
| `Window.ScreenToClient` / `ClientToScreen` | — | Later, with the image-return flow |
| Global `MoveMouseTo` / `ClickMouseAt` / right / middle / double / `ScrollWheel` | `move_mouse_to` / `click_at` / `double_click_at` / `scroll_wheel` | Covered |
| Global `KeyDown` / `KeyUp` / `Press` / `PressHotkey` / `Type` / `PressFor` | — | Later |
| Global `MouseDown` / `MouseUp` / `MoveMouseBy` / `MoveMouseToExact` | — | Later |
| `SetBackend` | `switch_backend` | Covered |
| `GetCursorPos` / `screen.VirtualBounds` | `get_cursor_pos` / `get_virtual_bounds` | Covered |
| `screen.CaptureVirtualDesktop` | `capture_screen` | Covered after the DPI prerequisite |
| `screen.CaptureRegion` / `Monitors` / `ImageToVirtual` | — | Later, with the image-return flow |
| `Window.DPI` / `ClientRect` / `IsValid` / `IsVisible` | `find_window` / `find_child` metadata | Covered |

The largest remaining gap is the image-return flow: screen capture returns dimensions only, so there is no "locate on screen, then click" loop inside MCP yet. The input side of that loop is complete.

## Scope

| | Content |
|---|---|
| **Now** | `capture_screen` DPI prerequisite; screen-coordinate input family (`move_mouse_to`, `click_at`, `double_click_at`, `scroll_wheel`); window-directed `scroll`; `find_window` `pid` + `index` + `count`; complete key-name contract |
| **Later** | Image return / `capture_region`; global keyboard family; `MouseDown` / `MouseUp` / `MoveMouseBy` / `MoveMouseToExact`; `ScreenToClient` / `ClientToScreen`; `screen.Monitors` / `ImageToVirtual`; `Window.MoveRel`; `list_windows` |
| **Never** | OCR/CV/DOM automation, agent runtime, skills/planning; HWND or other provider-native handles in the MCP contract; MCP-specific interfaces in the core library |

**Completion criterion (Q1):** when the caller already holds virtual-desktop coordinates, MCP can move, click, double-click, and scroll. The adapter does not discover screen targets, and `capture_screen` does not carry that responsibility.

The "now" items were accepted because each maps directly to an existing exported function, needs no new abstraction, and has a concrete acceptance check. They also fix real determinism defects: list searches that did not report candidate counts, `capture_screen` failing in the default process, and an overly narrow key set.

## Tool Set

The catalog exposes 19 stable low-level tools.

Initial tool set:
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

Gap-fill additions:
- `move_mouse_to`
- `click_at`
- `double_click_at`
- `scroll_wheel`
- `scroll`

For target-discovery tools such as `find_window` and `find_child`, the adapter also returns stable runtime metadata when available:
- visibility
- client area size
- DPI
- requested lookup hints

## Tool Contract Requirements

Each MCP tool provides:
- stable `name`
- concise `description`
- JSON Schema `inputSchema`
- JSON Schema `outputSchema`
- structured error codes
- side-effect metadata
- confirmation requirement metadata where needed

The server keeps the runtime wire shape stable by using fixed response envelopes and typed structured payloads rather than ad hoc `map[string]any` payloads.

Recommended side-effect levels:
- `read_only`
- `state_change`
- `sensitive`
- `destructive`

Error codes:
- `validation_error`
- `permission_denied`
- `target_not_found`
- `target_not_visible`
- `timeout`
- `backend_unavailable`
- `unsafe_operation`
- `internal_error`

### Schema and Call-Time Enforcement (Q13)

Constraints introduced by the gap-fill work are enforced on the `tools/call` path, not only in the JSON Schema:

- A missing required coordinate or `delta` on a new tool returns `validation_error`.
- An explicit JSON `null` for a required argument also returns `validation_error`. `decodeArgs` converts `null` to a zero value, so the raw `arguments` map is checked for presence and non-null before dispatch.
- An explicit zero is a legal value. Coordinates `(0, 0)` and `delta: 0` are not the same as omission.
- `press_hotkey` rejects an empty or null `keys` array and rejects more than 8 keys with `validation_error`.
- `scroll_wheel` relies on the library's own zero rejection for a missing `steps`; it does not add a presence check.
- Existing tools keep their previous zero-value behavior. This is additive and does not break published calls.
- `pid` and `index` keep "omitted and 0 are the same".

## Gap-Fill Tool Designs

### `capture_screen` DPI Prerequisite (Q7)

- **Problem:** `screen.CaptureVirtualDesktop` returns an error when `window.IsPerMonitorDPIAware()` is false. `cmd/mcp-server/main.go` did not enable per-monitor DPI awareness, so the default process always failed.
- **Design:** call `winput.EnablePerMonitorDPI()` once at the `cmd/mcp-server/main.go` entrypoint. Do not put it in `NewServer`, because that would change the DPI awareness of the `go test` process.
- **Success criterion:** after the call, `window.IsPerMonitorDPIAware() == true`; a nil return is not enough. `EnablePerMonitorDPI` can fall back to system-level `SetProcessDPIAware` and still return nil, while `IsPerMonitorDPIAware()` remains false and `CaptureVirtualDesktop` keeps failing.
- **Failure policy:** if awareness is still false, log a warning and keep serving. `capture_screen` then fails with the library error.
- **Output contract:** unchanged; width and height only.
- **Acceptance:** starting the server in a default process and calling `capture_screen` returns non-zero width and height, with no `not Per-Monitor DPI Aware` error.

### `move_mouse_to(x, y)` (Q9, Q13)

- **Mapping:** `winput.MoveMouseTo(x, y)`.
- **Input:** `x` and `y` are required integers in absolute virtual-desktop coordinates, the same coordinate space as `get_cursor_pos` and `get_virtual_bounds`. Missing or null returns `validation_error`; an explicit `0` is legal.
- **Output:** `okSchema`.
- **Metadata:** `state_change`, confirmation `true`.
- **Bounds:** before calling the library, check the half-open rectangle `Left <= x < Right && Top <= y < Bottom` using `screen.VirtualBounds()`. This matches `hid.insideVirtualBounds`. The virtual desktop origin may be negative, so points left of or above the primary monitor are legal. A point outside the rectangle returns `validation_error`.
- **Trade-off:** map `MoveMouseTo`, not `MoveMouseToExact`; the latter adds a 12 ms wait and convergence check and remains a later option.
- **Acceptance:** table-driven bounds test including negative origins; schema assertion; real-desktop smoke test.

### `click_at(x, y, button)` (Q9, Q13)

- **Mapping:** `button` is `"left"` (default), `"right"`, or `"middle"` and maps to `winput.ClickMouseAt`, `ClickRightMouseAt`, or `ClickMiddleMouseAt`. An invalid button returns `validation_error`.
- **Input:** `x` and `y` required; missing or null returns `validation_error`; explicit `0` is legal. `button` is a string enum.
- **Output:** `okSchema`.
- **Metadata:** `state_change`, confirmation `true`.
- **Bounds:** same half-open virtual-desktop pre-check as `move_mouse_to`; outside returns `validation_error`.
- **Acceptance:** table-driven button dispatch test through the extracted `resolveClickAt` seam; schema assertion.

### `double_click_at(x, y)` (Q9, Q13)

- **Mapping:** `winput.DoubleClickMouseAt(x, y)`.
- **Input:** `x` and `y` required; missing or null returns `validation_error`; explicit `0` is legal.
- **Output:** `okSchema`.
- **Metadata:** `state_change`, confirmation `true`.
- **Determinism:** keep it symmetric with the window-targeted `double_click` instead of adding a `double` boolean to `click_at`. One tool carries one semantic, matching the rest of the catalog.
- **Bounds:** same half-open virtual-desktop pre-check; outside returns `validation_error`.

### `scroll_wheel(steps)` (Q8, Q13)

- **Mapping:** `winput.ScrollWheel(steps int)`. One step is one wheel notch; positive scrolls forward. The library rejects `0` and `|steps| > 273` with `ErrInvalidScrollSteps`.
- **Input:** `steps` is a required integer. A missing value decodes to 0 and the library rejects it, so no separate presence check is added.
- **Output:** `okSchema`.
- **Error:** `ErrInvalidScrollSteps` maps to `validation_error`; other errors use `mapLibraryError`.
- **Metadata:** `state_change`, confirmation `true`.
- **Description:** no coordinate arguments; the event goes to the window under the current cursor, so the caller positions the cursor with `move_mouse_to` first.
- **Units (Q8):** `scroll_wheel` counts whole steps, while `scroll` uses Win32 increments of 120 per notch. The adapter does not rewrite the library's units or no-op semantics.
- **Acceptance:** invalid `steps` returns `validation_error`; schema assertion.

### `scroll(target_id, x, y, delta)` (Q8, Q13)

- **Mapping:** `Window.Scroll(x, y, delta)` with client-area coordinates. `delta` is a Win32 wheel increment; one notch is 120, and the direction matches `scroll_wheel`.
- **Input:** `target_id`, `x`, `y`, and `delta` are required. Missing or null `x`/`y`/`delta` returns `validation_error`. An explicit `delta: 0` remains accepted by the presence check.
- **Output:** `okSchema`.
- **No-op semantics:** on the message backend, `delta == 0` is a successful no-op, because `mouse.Scroll` returns nil before posting. On the HID backend, `Window.Scroll` returns `ErrDirectedScrollUnsupported` before the zero check, so even `delta: 0` returns `backend_unavailable`. The description states this qualification.
- **Errors:** invalid `target_id` returns `target_not_found`; `ErrInvalidScrollDelta` (delta outside int16) returns `validation_error`; `ErrDirectedScrollUnsupported` returns `backend_unavailable`.
- **Metadata:** `state_change`, confirmation `true`; description states message-backend availability, HID behavior, and the 120-per-notch unit.
- **Acceptance:** error-mapping tests for `ErrInvalidScrollDelta` and the HID branch; explicit-zero behavior test; schema assertion.

### `find_window`: pid, index, count (Q2–Q5, Q11, Q12)

- **`pid` (optional integer):** selects candidates via `winput.FindByPID`.
- **`index` (optional integer, default 0):** selects the Nth candidate. An out-of-range value returns `validation_error`.
- **`count` (optional output integer, Q2):** reported only for `pid` and `process_name` candidate lists. `title` and `class` use `FindWindowW`, which sees one window per call; reporting `count` there would misrepresent "one visible match" as "unique on the desktop". Omitted `count` means "this search cannot see duplicates".
- **Selector precedence (Q5):** `title > class > process_name > pid`. Two or more non-empty selectors are not an error and are not intersected; lower-priority selectors are ignored. The description states the order. Existing successful calls keep their meaning.
- **Multiple candidates (Q4):** omitted `index` means 0. `count > 1` is a disambiguation signal, not a failure; existing `process_name` calls keep succeeding. Only an out-of-range index is `validation_error`.
- **`index` with title/class (Q11):** there is no candidate list. Omitted `index` or `0` selects the returned window; any other value, including a negative one, returns `validation_error`. `count` is still not reported.
- **`pid` empty/zero (Q12):** omitted and `0` both mean "no process ID" (PID 0 is Idle and is never a lookup target). Legal values are 1–4294967295 (uint32). The field is `uint32`, so negative, non-integer, or above-uint32 values fail decoding with `validation_error`, even when another selector is present. An ignored `pid` is accepted when it is an integer; `0` is treated as omitted.
- **Candidate identity (Q3):**
  - `pid` candidates come from the current `EnumWindows` Z-order. Hidden top-level windows are included, and the order changes when focus changes.
  - `process_name` collects top-level windows from **every** process whose executable name matches, through `window.FindPIDsByName` (Toolhelp snapshot order). Within each PID, windows follow `EnumWindows` Z-order. A same-named process with no top-level window is skipped.
  - No window identity field is added and `list_windows` is not implemented. The returned `title`, `class`, and `process_name` are request echoes, not the window's own identity. To identify a concrete window, bind the returned `target_id` and call `read_text`.
- **Compatibility:** `title`/`class` keep their single-result semantics; `process_name`/`pid` use the candidate list + `index`; all existing fields, types, and required sets are unchanged. This is additive under the Compatibility Strategy.
- **Selection seam:** `pickCandidate(list, index)` is a pure function so the index rules are testable without Win32.
- **Acceptance:** table-driven candidate/index tests; schema assertions for `pid`/`index`/`count`; description assertions for Z-order, all same-named processes, selector order, and the `read_text` hint.

### `press_key` / `press_hotkey` Key Names (Q6, Q10, Q13)

- **Current contract:** `keyFromName` matches one canonical short name per key, case-insensitively, after trimming whitespace. Published names (`ENTER`, `TAB`, `ESC`, `SPACE`, `CTRL`, `SHIFT`, `ALT`, `A`–`Z`, `UP`, `DOWN`, `LEFT`, `RIGHT`) are unchanged. There are no aliases such as `ESCAPE`, `BACKSPACE`, `ARROWUP`, `LEFTCTRL`, or `DIVIDE`.
- **Single characters:** after short-name matching, a single character that `keyboard.LookupKey` reports with `Shifted == false` is accepted (`,` `.` `-` `;` `[` `]` `\` `'` `` ` `` `=` `/`). A character that needs Shift (`!`, `?`, `<`, `:`, `"`) returns `validation_error` and the description points to `press_hotkey`, for example `["SHIFT","1"]`. `KeyFromRune`/`LookupKey` discard the Shift bit, so pressing the bare key would produce the wrong character.
- **Short-name table:**

| Short names | Keys |
|---|---|
| `A`–`Z`, `0`–`9` | Letters and top-row digits |
| `ESC`, `F1`–`F12`, `ENTER`, `TAB`, `SPACE`, `BKSP` | Control keys |
| `CAPS`, `NUMLOCK`, `SCROLLLOCK` | Lock keys (scroll lock is not `SCROLL`) |
| `CTRL`, `SHIFT`, `ALT` | Left modifiers |
| `RIGHTCTRL`, `RIGHTSHIFT`, `RIGHTALT` | Right modifiers |
| `LWIN`, `RWIN` | Windows keys |
| `UP`, `DOWN`, `LEFT`, `RIGHT` | Arrow keys |
| `HOME`, `END`, `INSERT`, `DELETE`, `PAGEUP`, `PAGEDOWN` | Navigation keys |
| `MINUS`, `EQUAL`, `COMMA`, `DOT`, `SLASH`, `SEMI`, `QUOT`, `TICK`, `LBR`, `RBR`, `BACKSLASH` | Punctuation keys; the non-Shift single characters also work |
| `NUMPAD0`–`NUMPAD9`, `NUMPADDECIMAL`, `NUMPADMULTIPLY`, `NUMPADADD`, `NUMPADSUBTRACT`, `NUMPADDIVIDE`, `NUMPADENTER` | Numpad; `KeyDivide` and `NUMPADDIVIDE` are the same key and share one name |

- **Schema:** `press_key.key` and `press_hotkey.keys[]` stay strings with the full name list in the description. A giant enum would duplicate the library parser's responsibility.
- **Resource limit:** `press_hotkey.keys` has `minItems: 1` and `maxItems: 8`. Call time enforces both: an empty or null array and more than 8 keys return `validation_error`.
- **Acceptance:** exhaustive table-driven short-name test; non-Shift single-character test; Shift-character and unknown-name rejection; schema assertions for `minItems`/`maxItems`; description contains the full table and no rejected aliases.
- **Discovery:** `keyNamesDescription` is shared by both tool descriptions, so `tools/list` publishes the same table the parser accepts.

## Failure Policy

- A single tool call fails with a structured error (`code`, `retryable`, `details`) and does not stop the server.
- `validation_error`: bad input, missing/null required arguments, out-of-bounds coordinates, unsupported key names, invalid scroll steps/delta, unsupported mouse buttons.
- `target_not_found`: no window matched, or the handle is gone.
- `target_not_visible`: the target window is hidden or minimized.
- `permission_denied`: UIPI or foreground restrictions.
- `backend_unavailable`: driver/DLL failure, or directed scrolling on the HID backend.
- `unsafe_operation`: the tool's side-effect level is not enabled.
- `internal_error`: anything the adapter cannot classify.
- `capture_screen` DPI initialization failure is logged as a warning; the server keeps running and the call returns the library error.
- An empty or missing candidate list is `target_not_found`, never success.
- `ErrInvalidScrollSteps`, `ErrInvalidScrollDelta`, `ErrInvalidMouseCoordinate`, `ErrUnsupportedKey`, and `ErrOutsideVirtualDesktop` map to `validation_error`. `ErrDirectedScrollUnsupported` maps to `backend_unavailable`. `ErrWindowNotVisible` maps to `target_not_visible`.

## Known Limitations

- `capture_screen` returns dimensions only; the bitmap is discarded. Image return and region capture are later work.
- Screen-coordinate tools use absolute virtual-desktop coordinates. Client/screen conversion tools are later work; callers convert window client coordinates themselves.
- The global keyboard family (no window target) is later work; current keyboard tools require `target_id`.
- Directed scrolling is message-backend only. On HID, `scroll` returns `backend_unavailable`.
- Candidate order is not a stable identity. `pid` order follows the current Z-order; `process_name` follows Toolhelp snapshot order and then Z-order per PID.
- The in-memory `target_id` registry retains every resolved target for the life of the server process. There is no eviction or persistence; a long-running client should not resolve unbounded numbers of targets.

## Validation and Acceptance

| Item | Method | Pass criterion |
|---|---|---|
| Pure logic: candidate selection, index bounds, title/class with `index != 0`, pid 0 / above uint32 / non-integer, exhaustive key short names, non-Shift characters, Shift-character rejection, button dispatch, `minItems`/`maxItems`, missing/null `x`/`y`/`delta` rejection, explicit 0 acceptance, empty and 9-key `press_hotkey` rejection | Table-driven unit tests in `internal/mcpadapter` | All pass |
| New tool schemas: input/output/annotations/required/errorSchema | `tools/list` structural assertions | All pass |
| Mutation gating | Call each `state_change` tool with the default config | `unsafe_operation` |
| DPI prerequisite success | Start the server in a default process and check awareness | `capture_screen` returns non-zero dimensions, no DPI error |
| DPI prerequisite failure | Awareness remains false | Server keeps running; `capture_screen` returns the library DPI error |
| Real input | Manual smoke test on a real desktop (not CI) | Move/click/scroll/key work |

## Anti-Over-Engineering Review

- The screen input family keeps the same one-tool-per-action granularity as the window family (`move_mouse_to`, `click_at`, `double_click_at`, `scroll_wheel`) instead of one `mouse` tool with mode switches.
- `find_window` uses `index` + `count` instead of a new `list_windows` tool; the disambiguation cost is minimal.
- Global keyboard, coordinate conversion, and `MouseDown`/`MouseUp` stay out of "now"; the window family covers the main cases, and the rest only pay off with the image-return flow.
- The key extension reuses the library's `LookupKey` (including the Shift bit) and `Key` constants instead of inventing a giant schema enum.

## Public Boundary Rules

- MCP contracts use runtime-neutral target descriptions, not provider-native handles.
- Tool outputs are useful to external agents without exposing internals.
- The adapter normalizes library errors into machine-actionable MCP responses.
- Tool names stay stable over time.
- Breaking schema changes use a new spec major version or a new tool name.

## Suggested Directory Layout

Actual layout:

- `cmd/mcp-server`
  - MCP server entrypoint
  - DPI initialization
  - transport lifecycle
- `internal/mcpadapter`
  - tool catalog and schemas
  - argument validation
  - tool-to-library mappings
  - error mapping
  - confirmation policy
  - target registry
  - framing and protocol tests

- `docs/mcp-tools.md`
  - adapter design, gap-fill decisions, and boundary rules

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
- tool discovery through `tools/list`
- `tools/call` mappings for all 19 tools
- JSON Schema input and output metadata on each tool
- structured error metadata per tool
- in-memory `target_id` registry for window and child-window flows
- safety gates for mutating and sensitive tools
- framed `Content-Length` request and response transport with a bounded line/header reader
- fixed typed response envelopes for initialization, tool listing, tool results, and structured error data
- JSON-RPC `ping` handling and protocol version negotiation in `initialize`
- screen-coordinate input tools (`move_mouse_to`, `click_at`, `double_click_at`, `scroll_wheel`) with virtual-desktop bounds pre-check, and the window-directed wheel tool `scroll`
- `find_window` `pid` selector, `index` candidate selection, and `count` reporting for list-based searches
- expanded key short-name contract for `press_key` / `press_hotkey`, `minItems: 1` / `maxItems: 8`, and call-time presence, null, empty-array, and size validation
- per-monitor DPI awareness enabled at the entrypoint so `capture_screen` works in the default process

Example protocol usage is documented in `docs/mcp-example-session.md`.

## Decision Record — Third-Party MCP SDK (2026-11-06)

**Decision: keep the hand-rolled protocol layer; do not adopt `github.com/mark3labs/mcp-go` for now.**

Facts (evidence, not estimates):

- `go.mod` declares `go 1.22` and currently has **zero external dependencies**.
- Latest `mcp-go` release (v1.5.0, under active maintenance) requires **`go 1.25.5`** and pulls six direct dependencies (`google/jsonschema-go`, `google/uuid`, `santhosh-tekuri/jsonschema/v6`, `spf13/cast`, `yosida95/uritemplate/v3`, `golang.org/x/text`).
- The adapter is small, has framing / error / smoke tests, and covers a stable protocol surface (`initialize`, `initialized`, `tools/list`, `tools/call`, `ping`).
- The documented non-goals — no agent runtime, no OCR/CV/DOM, no resources/prompts — are exactly the capabilities `mcp-go` bundles for free.

Tradeoffs:

- **Gains from adopting**: protocol version negotiation, multiple transports (stdio / SSE / Streamable HTTP + OAuth), community-verified boundary behavior, and removing ownership of the protocol glue.
- **Costs from adopting**: ends the zero-dependency baseline, raises the Go toolchain floor to 1.25.5 (breaking downstream builds on older toolchains), and ships abstractions (resources/prompts/sampling/roots) the design explicitly rejects. The adapter's own contract — `sideEffectLevel` / `requiresConfirmation` annotations and structured `code` / `retryable` errors — is not part of the MCP spec, so it would have to be re-implemented on top of the SDK either way.
- The one real interop gap (missing `ping` handling) is a small local fix and is implemented, together with optional protocol version negotiation.

Re-evaluation triggers (revisit only on product evidence, not documentation inference):

1. A real client failing for lifecycle / `ping` / version reasons after this fix.
2. A concrete remote-transport demand (SSE / Streamable HTTP / OAuth).
3. A need for resources, prompts, `cancelled`, or progress that the current server deliberately omits.
4. MCP spec drift that actually breaks client handshakes.

## Compatibility Strategy

Use independent versioning for:
- the Go library API
- the MCP adapter spec

The MCP adapter evolves additively where possible:
- add tools without changing existing ones
- add optional fields without removing required ones
- reserve breaking schema changes for major version increments

## Change Log

- 2026-11-06: Q1–Q9 recorded — completion criterion, capability gaps, Q7 DPI, Q9 half-open bounds, Q8 scroll units, Q2–Q5 `find_window`, Q6 key contract.
- 2026-11-06: Q10–Q12 recorded — canonical key short names with no aliases; title/class reject `index != 0`; `pid` omission and `0` both mean unused, legal range 1–4294967295.
- 2026-11-06: Q13 recorded — new-tool required arguments are enforced at call time; `scroll_wheel` relies on the library's zero rejection; `press_hotkey` rejects more than 8 keys; existing tools keep their zero-value behavior.
- 2026-11-06: Implementation recorded — entrypoint DPI prerequisite and `IsPerMonitorDPIAware()` success check; screen input family with bounds pre-check; `scroll`; `find_window` `pid`/`index`/`count`; full key short-name table, non-Shift characters, Shift rejection, `maxItems: 8`; scroll error mappings; 13 unit tests; build/vet/test green; end-to-end smoke test of the 19-tool catalog, default-process capture, negative-origin bounds, and mutation gating.
- 2026-11-06: Review response — descriptions carry the full key table with rejection tests; DPI check precedes the warning; title/class index gaps covered; `find_window` description adds the `read_text` hint.
- 2026-11-09: Merged the standalone gap-fill document into this file and removed it. Resolved Q3 to match the implemented `FindByProcessName`: candidates come from every process with the matching executable name, not only the first PID. Fixed call-time validation so explicit `null` is rejected along with a missing key; `press_hotkey` now rejects empty arrays; added `minItems: 1`, schema, mutation-gating, key-table, pid-zero, header-cap, and blank-line regression tests. Request reading now skips blank lines iteratively and bounds header count and size. Qualified `scroll` `delta: 0` as a message-backend no-op. Added `ErrUnsupportedKey` / `ErrOutsideVirtualDesktop` / `ErrWindowNotVisible` error mappings.
