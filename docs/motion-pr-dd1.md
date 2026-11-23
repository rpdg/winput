# PR-DD1: motion extraction (pending independent review)

Base: `eb0022f49b5735662bc2b063e6ca453ca2599c59`.

`internal/motion` owns the existing human trajectory and corresponding button
phase algorithms, not a device or lifecycle manager. `Session` contains direct
relative-send and button-send callbacks valid only under the caller's current
phase lock. `Runtime` supplies cursor observations, `Sleep`, `After`, and `Intn`;
production still uses `window.GetCursorPos`, `time.Sleep`, `time.After`, and the
package-level `rand.Intn` (no private production RNG).

`hid.mouseMotion` is a private, per-call orchestration seam. Bounds are checked
before acquiring the movement phase; the whole trajectory runs inside that
phase. Every click finishes and unlocks movement, then independently calls
`acquireMouse` again for its button phase, permitting Close/reinitialization in
between. `mouseSession` captures that phase's ctx/dev and sends raw Interception
strokes without calling exported HID functions or taking another read lock.

For DoubleClick, SetCursorPos, its error, 12ms wait, GetDoubleClickTime, default
500ms and the duration-precision interval calculation remain in the adapter's
second locked phase. Only the deterministic four-button sequence is shared.
Ordinary Down has no retry; ordinary Up and all four double-click strokes
retain the one 6ms retry and first-error behavior. No fallback/cleanup Up was
added. Motion's terminal convergence error is supplied by the adapter so the
existing `hid.ErrMoveNotConverged` instance is returned unchanged. Other errors
are propagated without wrapping or remapping. The internal callbacks used by
an operation are required; there is no eager validation of unrelated callbacks.

The 3s timer is checked only at trajectory loop heads, not a total deadline.
Final convergence retains five wait/read/correct iterations and the immediate
post-fifth read. No DD step cap, delay, scaling or new backend is present.
MoveExact, one-shot MoveRelative/Button, keyboard humanSleep, root dispatch and
Interception DLL/lifecycle code are unchanged. `humanSleep` remains in HID for
keyboard Press; its original mouse formula also exists inside the pure core.

## Safe validation

- `go test ./internal/motion -count=1 -timeout=60s`: pure fake differential
  traces against a test-only frozen baseline, boundaries, failure identity,
  random draw order/endpoints, timers and fifth correction.
- On Linux, `go test ./hid/motion.go ./hid/motion_adapter_test.go -count=1
  -timeout=60s`: the real private orchestration, with local fake effects,
  verifying two phases, simulated Close/new handles, lock-spanning effects,
  entrance failures and exact double-click positioning/time order. This is a
  file-list test, **not** a Linux execution of the Windows HID package.
- Linux race can run on those same fake-only targets with an existing C compiler.
- Cross-compile Windows/amd64 with Go1.22, CGO=0; compile every test package to
  distinct scratch files. Compilation is not Windows runtime acceptance.
- `TestMotionSessionCapturedHandlesAndStrokes` and
  `TestMotionSessionPendingWriterNoRecursiveReadLock` in the Windows HID package
  use a fake raw sender; they do not call Init/load a DLL/send real input.
  The pending-writer test uses actual `initMutex` and `mouseSession`, with
  TryRLock as a barrier and bounded failure cleanup (no timing sleep guesses).
  They are compiled here, **not run on Windows**. Any later Windows test run
  needs separate authorization and a reviewed whitelist; do not run all HID
  tests as an input-safe suite.

No Windows/HID input, trajectory feel assessment or Windows race is claimed.
PR-DD1 remains pending those acceptance checks and independent review. This
change does not authorize DD2 or subsequent work.
