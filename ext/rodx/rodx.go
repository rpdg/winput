package rodx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/rpdg/winput"
)

const (
	defaultStartupTimeout  = 15 * time.Second
	defaultShutdownTimeout = 5 * time.Second
)

type Session struct {
	PID          uint32
	Browser      *rod.Browser
	Page         *rod.Page
	DebugURL     string
	selectorWait time.Duration
}

type ConnectOptions struct {
	DebugURL      string
	DebugPort     int
	TargetURLHint string
	TitleHint     string
	SelectorWait  time.Duration
}

type LaunchOptions struct {
	Executable     string
	Args           []string
	WorkingDir     string
	Env            []string
	DebugPort      int
	UserDataDir    string
	StartupTimeout time.Duration
	PageURLHint    string
	PageTitleHint  string
}

type RestartOptions struct {
	PID             uint32
	Executable      string
	Args            []string
	WorkingDir      string
	Env             []string
	DebugPort       int
	UserDataDir     string
	ShutdownTimeout time.Duration
	StartupTimeout  time.Duration
	PageURLHint     string
	PageTitleHint   string
	ForceKill       bool
}

type LaunchResult struct {
	PID      uint32
	DebugURL string
	Browser  *rod.Browser
	Page     *rod.Page
	Cmd      *exec.Cmd
	Session  *Session
}

type versionResponse struct {
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

func ConnectByWindow(w *winput.Window, opts ConnectOptions) (*Session, error) {
	if w == nil || !w.IsValid() {
		return nil, winput.ErrWindowGone
	}
	pid, err := pidFromHWND(w.HWND)
	if err != nil {
		return nil, err
	}
	return ConnectByPID(pid, opts)
}

func ConnectByPID(pid uint32, opts ConnectOptions) (*Session, error) {
	if pid == 0 {
		return nil, fmt.Errorf("invalid pid")
	}

	debugURL, err := normalizeDebugURL(opts.DebugURL, opts.DebugPort)
	if err != nil {
		return nil, err
	}

	timeout := opts.SelectorWait
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	browser, page, err := connectAndSelectPage(debugURL, opts.TargetURLHint, opts.TitleHint)
	if err != nil {
		return nil, err
	}

	return &Session{
		PID:          pid,
		Browser:      browser,
		Page:         page,
		DebugURL:     debugURL,
		selectorWait: timeout,
	}, nil
}

func LaunchWithDebugging(opts LaunchOptions) (*LaunchResult, error) {
	if opts.Executable == "" {
		return nil, ErrExecutableNotFound
	}
	if opts.DebugPort <= 0 {
		return nil, ErrDebugPortRequired
	}

	args, err := injectRemoteDebuggingArg(opts.Args, opts.DebugPort, opts.UserDataDir)
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(opts.Executable, args...)
	cmd.Dir = opts.WorkingDir
	if cmd.Dir == "" {
		cmd.Dir = workingDirFromExecutable(opts.Executable)
	}
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	} else {
		cmd.Env = os.Environ()
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrExecutableNotFound, err)
	}

	debugURL, err := waitForDebugEndpoint(opts.DebugPort, normalizeDuration(opts.StartupTimeout, defaultStartupTimeout))
	if err != nil {
		return nil, err
	}

	browser, page, err := connectAndSelectPage(debugURL, opts.PageURLHint, opts.PageTitleHint)
	if err != nil {
		return nil, err
	}

	session := &Session{
		PID:          uint32(cmd.Process.Pid),
		Browser:      browser,
		Page:         page,
		DebugURL:     debugURL,
		selectorWait: 5 * time.Second,
	}

	return &LaunchResult{
		PID:      uint32(cmd.Process.Pid),
		DebugURL: debugURL,
		Browser:  browser,
		Page:     page,
		Cmd:      cmd,
		Session:  session,
	}, nil
}

func RestartWithDebugging(opts RestartOptions) (*LaunchResult, error) {
	if opts.PID == 0 {
		return nil, ErrRestartSpecIncomplete
	}
	if opts.DebugPort <= 0 {
		return nil, ErrDebugPortRequired
	}

	executable := opts.Executable
	if executable == "" {
		var err error
		executable, err = executableFromPID(opts.PID)
		if err != nil {
			return nil, err
		}
	}

	closeWindowsForPID(opts.PID)
	err := waitForExit(opts.PID, normalizeDuration(opts.ShutdownTimeout, defaultShutdownTimeout))
	if err != nil {
		if !opts.ForceKill {
			return nil, err
		}
		if killErr := terminatePID(opts.PID); killErr != nil {
			return nil, killErr
		}
		if waitErr := waitForExit(opts.PID, normalizeDuration(opts.ShutdownTimeout, defaultShutdownTimeout)); waitErr != nil {
			return nil, waitErr
		}
	}

	return LaunchWithDebugging(LaunchOptions{
		Executable:     executable,
		Args:           opts.Args,
		WorkingDir:     firstNonEmpty(opts.WorkingDir, workingDirFromExecutable(executable)),
		Env:            opts.Env,
		DebugPort:      opts.DebugPort,
		UserDataDir:    opts.UserDataDir,
		StartupTimeout: opts.StartupTimeout,
		PageURLHint:    opts.PageURLHint,
		PageTitleHint:  opts.PageTitleHint,
	})
}

func (s *Session) ValueBySelector(selector string) (string, error) {
	el, err := s.elementBySelector(selector)
	if err != nil {
		return "", err
	}
	return valueFromElement(el)
}

func (s *Session) ValueByXPath(xpath string) (string, error) {
	page := s.pageWithTimeout()
	el, err := page.ElementX(xpath)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSelectorNotFound, err)
	}
	return valueFromElement(el)
}

func (s *Session) Eval(js string) (string, error) {
	res, err := s.Page.Eval(js)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDOMReadFailed, err)
	}
	return res.Value.Str(), nil
}

func (s *Session) Close() error {
	if s == nil || s.Browser == nil {
		return nil
	}
	return s.Browser.Close()
}

func (s *Session) pageWithTimeout() *rod.Page {
	if s.selectorWait <= 0 {
		return s.Page
	}
	return s.Page.Timeout(s.selectorWait)
}

func (s *Session) elementBySelector(selector string) (*rod.Element, error) {
	el, err := s.pageWithTimeout().Element(selector)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSelectorNotFound, err)
	}
	return el, nil
}

func valueFromElement(el *rod.Element) (string, error) {
	prop, err := el.Property("value")
	if err == nil && !prop.Nil() {
		return prop.Str(), nil
	}

	text, err := el.Text()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrDOMReadFailed, err)
	}
	return text, nil
}

func normalizeDebugURL(debugURL string, port int) (string, error) {
	if debugURL != "" {
		return strings.TrimRight(debugURL, "/"), nil
	}
	if port <= 0 {
		return "", ErrDebugPortRequired
	}
	return fmt.Sprintf("http://127.0.0.1:%d", port), nil
}

func injectRemoteDebuggingArg(args []string, port int, userDataDir string) ([]string, error) {
	out := make([]string, 0, len(args)+2)
	for _, arg := range args {
		if strings.HasPrefix(arg, "--remote-debugging-port") {
			return nil, ErrDebugPortConflict
		}
		out = append(out, arg)
	}
	out = append(out, fmt.Sprintf("--remote-debugging-port=%d", port))
	if userDataDir != "" {
		out = append(out, "--user-data-dir="+userDataDir)
	}
	return out, nil
}

func waitForDebugEndpoint(port int, timeout time.Duration) (string, error) {
	debugURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		if ok, err := debugEndpointReady(debugURL); ok {
			return debugURL, nil
		} else if err != nil && ctx.Err() != nil {
			return "", fmt.Errorf("%w: %v", ErrDebugEndpointTimeout, err)
		}

		select {
		case <-ctx.Done():
			return "", ErrDebugEndpointTimeout
		case <-ticker.C:
		}
	}
}

func debugEndpointReady(debugURL string) (bool, error) {
	resp, err := http.Get(strings.TrimRight(debugURL, "/") + "/json/version")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("status %d", resp.StatusCode)
	}

	var version versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return false, err
	}
	return version.WebSocketDebuggerURL != "", nil
}

func connectAndSelectPage(debugURL string, urlHint string, titleHint string) (*rod.Browser, *rod.Page, error) {
	browser := rod.New().ControlURL(debugURL).NoDefaultDevice()
	if err := browser.Connect(); err != nil {
		return nil, nil, fmt.Errorf("%w: %v", ErrBrowserConnectFailed, err)
	}

	page, err := selectPage(browser, urlHint, titleHint)
	if err != nil {
		_ = browser.Close()
		return nil, nil, err
	}
	return browser, page, nil
}

func selectPage(browser *rod.Browser, urlHint string, titleHint string) (*rod.Page, error) {
	pages, err := browser.Pages()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTargetPageNotFound, err)
	}
	if len(pages) == 0 {
		return nil, ErrTargetPageNotFound
	}

	filtered := make([]*rod.Page, 0, len(pages))
	for _, page := range pages {
		info, infoErr := page.Info()
		if infoErr != nil {
			continue
		}
		if urlHint != "" && !strings.Contains(info.URL, urlHint) {
			continue
		}
		if titleHint != "" && !strings.Contains(info.Title, titleHint) {
			continue
		}
		filtered = append(filtered, page)
	}

	switch len(filtered) {
	case 0:
		if urlHint != "" || titleHint != "" {
			return nil, ErrTargetPageNotFound
		}
		if len(pages) == 1 {
			return pages[0], nil
		}
		return nil, ErrAmbiguousTargetPage
	case 1:
		return filtered[0], nil
	default:
		return nil, ErrAmbiguousTargetPage
	}
}

func normalizeDuration(got time.Duration, fallback time.Duration) time.Duration {
	if got <= 0 {
		return fallback
	}
	return got
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
