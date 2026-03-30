# rodx

`rodx` is the optional browser automation extension for `winput`.

It combines:

- `github.com/rpdg/winput` for window discovery
- `github.com/go-rod/rod` for Chromium / Electron DOM access

Install it only when you need browser-level automation:

```bash
go get github.com/rpdg/winput/ext/rodx
```

## What It Solves

Use `rodx` when:

- `winput.Text()` or `winput.Value()` can't read a Chromium / Electron-hosted input
- the target app exposes a DevTools / remote debugging endpoint
- you want to read DOM values by CSS selector or XPath

`rodx` does not replace `winput`.
The intended flow is:

1. use `winput` to find the target window or PID
2. use `rodx` to connect to the browser layer
3. read values from the DOM

## Main APIs

```go
func ConnectByWindow(w *winput.Window, opts ConnectOptions) (*Session, error)
func ConnectByPID(pid uint32, opts ConnectOptions) (*Session, error)

func LaunchWithDebugging(opts LaunchOptions) (*LaunchResult, error)
func RestartWithDebugging(opts RestartOptions) (*LaunchResult, error)

func (s *Session) ValueBySelector(selector string) (string, error)
func (s *Session) ValueByXPath(xpath string) (string, error)
func (s *Session) Eval(js string) (string, error)
func (s *Session) Close() error
```

## Example: Connect to an Existing Debug Port

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/rpdg/winput/ext/rodx"
)

func main() {
	session, err := rodx.ConnectByPID(12345, rodx.ConnectOptions{
		DebugPort:    9222,
		SelectorWait: 5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	value, err := session.ValueBySelector("input")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("input value:", value)
}
```

## Example: Bridge from `winput.Window`

```go
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/rpdg/winput"
	"github.com/rpdg/winput/ext/rodx"
)

func main() {
	w, err := winput.FindByTitle("My Electron App")
	if err != nil {
		log.Fatal(err)
	}

	session, err := rodx.ConnectByWindow(w, rodx.ConnectOptions{
		DebugPort:     9222,
		TitleHint:     "Login",
		SelectorWait:  5 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer session.Close()

	username, err := session.ValueBySelector("input[name='username']")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("username:", username)
}
```

## Example: Launch Chromium with Remote Debugging

```go
package main

import (
	"fmt"
	"log"

	"github.com/rpdg/winput/ext/rodx"
)

func main() {
	result, err := rodx.LaunchWithDebugging(rodx.LaunchOptions{
		Executable:   `C:\Program Files\Google\Chrome\Application\chrome.exe`,
		DebugPort:    9222,
		Args:         []string{"https://example.com"},
		PageURLHint:  "example.com",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer result.Session.Close()

	title, err := result.Session.Eval("() => document.title")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("title:", title)
}
```

## Value Reading Behavior

`ValueBySelector` uses this order:

1. read the DOM `value` property
2. if `value` is empty or unavailable, fall back to visible text

That makes it suitable for:

- `input`
- `textarea`
- some text containers

## Notes

- `ConnectByWindow` and `ConnectByPID` currently require `DebugURL` or `DebugPort`.
- `LaunchWithDebugging` and `RestartWithDebugging` are the helpers that actually create the remote debugging endpoint.
- If the target Electron / Chromium app does not expose a DevTools endpoint, `rodx` cannot read the DOM.

## Troubleshooting

### 1. `ErrDebugPortRequired`

`ConnectByWindow` and `ConnectByPID` do not auto-discover a debugging endpoint yet.

You must provide one of:

- `ConnectOptions.DebugURL`
- `ConnectOptions.DebugPort`

If you don't already have a running endpoint, use `LaunchWithDebugging` or `RestartWithDebugging`.

### 2. `ErrDebugEndpointTimeout`

This usually means the browser process did not expose DevTools in time.

Check:

- the process really accepted `--remote-debugging-port=<port>`
- the chosen port is not already occupied by another process
- the target app is actually Chromium / Electron-based
- the app did not filter or ignore the injected launch argument

Useful manual check:

```text
http://127.0.0.1:9222/json/version
```

If that URL does not respond with DevTools metadata, `rodx` will not be able to connect.

### 3. `ErrTargetPageNotFound`

The browser connection succeeded, but no page matched your hints.

Common causes:

- `PageURLHint` or `TargetURLHint` is too strict
- `PageTitleHint` or `TitleHint` does not match the actual page title
- the app is showing a different startup page than expected

Try:

- remove the hint and see if there is only one page
- loosen the URL fragment you match on
- use title matching only when URL is unstable

### 4. `ErrAmbiguousTargetPage`

More than one page matched, and `rodx` refused to guess.

Typical cases:

- multiple tabs are open
- Electron has more than one WebContents
- a login popup and the main page both match loosely

Fix it by adding:

- `TargetURLHint` or `PageURLHint`
- `TitleHint` or `PageTitleHint`

Use the most stable discriminator available, usually URL first, then title.

### 5. `ErrSelectorNotFound`

The DevTools connection worked, but the DOM query did not find your element.

Common causes:

- selector is wrong
- the element is rendered later than expected
- the target is inside an iframe
- the target is inside shadow DOM

What to try:

- increase `SelectorWait`
- verify the selector in Chrome DevTools manually
- if the target is in an iframe, switch to a frame-aware approach in raw `rod`
- if the target is in shadow DOM, `ValueBySelector` may not be enough and you may need a custom `Eval`

### 6. `ErrDOMReadFailed`

The element was found, but reading its value failed.

Typical causes:

- the target is not a normal form field
- the value lives in framework state instead of a plain DOM property
- the content is produced by custom rendering logic

In those cases, prefer:

```go
value, err := session.Eval(`() => {
  const el = document.querySelector("input")
  return el ? el.value : ""
}`)
```

### 7. Electron App Still Doesn't Expose DOM

Some Electron apps do not expose a usable remote debugging endpoint at all.

Typical reasons:

- production build disables DevTools
- startup wrapper strips Chromium flags
- the app launches child processes in a way that your restart options did not reproduce

When that happens:

- `winput.Text()` / `winput.Value()` may still work if the control is visible through Win32 or UIA
- otherwise you need an app-specific integration path rather than generic `rodx`
