package hid

import (
	"github.com/rpdg/winput/internal/motion"
	"time"
)

// mouseMotion is the small per-call Interception orchestration seam. In
// production acquire is acquireMouse + a callback bundle bound to its handles;
// tests replace only these effects, not the orchestration or motion algorithms.
// No Session survives its unlock or gets reused for a second phase.
type mouseMotion struct {
	runtime               motion.Runtime
	inside                func(int32, int32) bool
	acquire               func() (motion.Session, func(), error)
	setCursor             func(int32, int32) error
	doubleClickTime       func() time.Duration
	outside, notConverged error
}

func (m mouseMotion) move(x, y int32) error {
	if !m.inside(x, y) {
		return m.outside
	}
	s, unlock, err := m.acquire()
	if err != nil {
		return err
	}
	defer unlock()
	return motion.Move(s, m.runtime, x, y, m.notConverged)
}

func (m mouseMotion) click(x, y int32, b motion.Button) error {
	if err := m.move(x, y); err != nil {
		return err
	}
	s, unlock, err := m.acquire()
	if err != nil {
		return err
	}
	defer unlock()
	return motion.Click(s, m.runtime, b)
}

func (m mouseMotion) doubleClick(x, y int32) error {
	if err := m.move(x, y); err != nil {
		return err
	}
	s, unlock, err := m.acquire()
	if err != nil {
		return err
	}
	defer unlock()
	if err := m.setCursor(x, y); err != nil {
		return err
	}
	m.runtime.Sleep(12 * time.Millisecond)
	sysDc := m.doubleClickTime()
	if sysDc == 0 {
		sysDc = 500 * time.Millisecond
	}
	interval := sysDc / 3
	if interval < 30*time.Millisecond {
		interval = 30 * time.Millisecond
	}
	return motion.DoubleClick(s, m.runtime, interval)
}
