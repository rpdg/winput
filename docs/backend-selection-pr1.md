# Backend selection refactor (PR1, unreleased)

## Observable changes

The intended observable changes are:

1. `SetBackend` prepares HID without holding `inputMutex` or `backendMutex`,
   then commits under `inputMutex` → `backendMutex`. It waits for the complete
   current input transaction (Type, hotkey, click, or clipboard swap/key/restore)
   and briefly blocks fixed `Input` calls as well. Concurrent commits follow
   lock acquisition, not call start order. Same-kind calls still prepare first;
   preparation failure does not change the selected backend.
2. Bugfix: unknown kinds such as `Backend(42)` consistently follow Message
   input, including Window.Type, window VK preflight, and modifier-hotkey
   foreground promotion. SetBackend still accepts/preserves unknown integers;
   NewInput still rejects them (including 2 and negative values).
3. Foreground modifier-hotkey bugfix for window calls (including fixed window
   facades): use ordinary `SetForegroundWindow`, without the Force/AttachThreadInput
   fallback. An initial foreground of 0 is rejected before activation or input.
   Activation and restoration require queue acknowledgement and foreground
   confirmation; the target ancestor and foreground are rechecked before each
   key-down. Observed mismatches prevent further downs; pressed keys are still
   cleaned up before restoration.
   Each confirmation has a 500ms budget. Restoration shares its budget between
   acknowledging the target's pending activation and confirming the previous
   foreground. These are confirmation budgets, not hard wall-clock limits on
   native calls, scheduling or externally shared input queues.
   Restoration remains best effort: a timeout cannot cancel pending activation,
   and failed target acknowledgement is reported as unconfirmed even if the
   previous foreground currently matches. External focus changes and input are
   not atomic; this does not promise absolute restoration or application receipt.

Global input copies one complete kind/device selection while holding the input
transaction lock, releases the backend read lock, then checks availability and
uses only that copy. Fixed Input keeps its construction selection and adds no
runtime wrapper preflight. No public API or error sentinel is added; the
foreground-hotkey failure conditions and messages change as described above.
Clipboard restore is still synchronous within the transaction, not an
application-consumption ACK.

## Preserved behavior and scope

The private nine-operation device interface has a stateless, one-to-one
Interception adapter. Message HWND/system paths remain in the root package.
Window.MoveRel still reads the cursor and calls MoveTo; MoveMouseBy alone uses
MoveRelative. Current-position click still uses button Down/wait/Up. Exact
movement keeps root preflight plus the low-level checks. Coordinates, lifecycle
phases, trajectories, retries and timing in `hid/` are untouched.

Device Type retains full LookupKey preflight, foreground checks before Shift
and character Down, the 10ms Shift wait, fixed hold, explicit-only intercharacter
gap, no final gap and reverse-order release/retry/error matching. Internal key
sends use hwnd=0 after those explicit checks, avoiding duplicate target checks.
Foreground-hotkey sending has its own test seam and keeps cleanup before
foreground restoration. Win is still not a modifier for promotion purposes.

## Test evidence and limitations

`backend_pr1_test.go` adds controlled fake tests for preparation, atomic
selection, transaction/restore serialization, unknown-kind paths, Type event
and wait sequences, foreground-hotkey cleanup and mouse dispatch. Existing
Input, probe, Paste and internal tests are migrated, not removed or weakened.
All global test seams are restored; these tests must not run in parallel.

Linux validation: cross-build, cross-vet and Windows test compilation are not
Windows test execution. The external delivery report records command results,
baseline vet warnings and any source-extracted native fake harness separately.
A source-extracted harness uses the selected unchanged Go bodies with OS/driver
stubs; it demonstrates only fake sequence/lock logic, not Windows behavior.

Required follow-up on an isolated Windows test desktop: execute the full suite,
run race with its own supported toolchain, verify Go 1.22, and perform qualified
Interception/multimonitor/DPI/foreground/clipboard/input checks. No driver is
installed and no production desktop input is injected by the Linux validation.
Windows native and race acceptance for the foreground fix remains open. Historical
results for `96a1556cd2ed6f1141e4407b5d825fffe7d8b3d9` do not validate these changed
bytes. Independent source review is not runtime acceptance and does not authorize
merge or later work.
