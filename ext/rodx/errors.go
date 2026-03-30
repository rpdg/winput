package rodx

import "errors"

var (
	ErrDebugPortRequired     = errors.New("debug port or debug url is required")
	ErrDebugPortConflict     = errors.New("remote debugging port already present in arguments")
	ErrExecutableNotFound    = errors.New("executable not found")
	ErrDebugEndpointTimeout  = errors.New("debug endpoint did not become ready before timeout")
	ErrBrowserConnectFailed  = errors.New("failed to connect to browser debugging endpoint")
	ErrRestartSpecIncomplete = errors.New("restart specification is incomplete")
	ErrProcessShutdownFailed = errors.New("failed to shut down target process")
	ErrTargetPageNotFound    = errors.New("target page not found")
	ErrAmbiguousTargetPage   = errors.New("multiple candidate pages matched and hints were insufficient")
	ErrSelectorNotFound      = errors.New("selector not found")
	ErrDOMReadFailed         = errors.New("failed to read DOM value")
)
