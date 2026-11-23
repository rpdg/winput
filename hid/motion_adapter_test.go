package hid

// Run on Linux without importing the Windows HID package:
// go test ./hid/motion.go ./hid/motion_adapter_test.go
// These tests exercise the SAME private orchestration used by the exported HID
// entry points. No system cursor, DLL, keyboard or input API is called.
import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rpdg/winput/internal/motion"
)

type adapterFixture struct {
	t                       *testing.T
	trace                   []string
	phase, held, generation int
	x, y                    int32
	acquireFail             int
	outside                 bool
	cursorFail              bool
	setFail                 bool
	dc                      time.Duration
}

func (f *adapterFixture) log(s string) { f.trace = append(f.trace, s) }
func (f *adapterFixture) locked() {
	f.t.Helper()
	if f.held == 0 {
		f.t.Fatal("effect outside lifecycle lock", f.trace)
	}
}
func (f *adapterFixture) adapter() mouseMotion {
	return mouseMotion{
		runtime: motion.Runtime{
			Cursor: func() (int32, int32, error) {
				f.locked()
				f.log("cursor")
				if f.cursorFail {
					return 0, 0, errAdapter
				}
				return f.x, f.y, nil
			},
			Sleep: func(d time.Duration) { f.locked(); f.log("sleep " + d.String()) },
			Intn:  func(n int) int { f.locked(); f.log(fmt.Sprintf("rand %d", n)); return n / 2 },
			After: func(d time.Duration) <-chan time.Time {
				f.locked()
				f.log("timer " + d.String())
				return make(chan time.Time)
			},
		},
		inside: func(x, y int32) bool {
			if f.held != 0 {
				f.t.Fatal("bounds under lock")
			}
			f.log("bounds")
			return !f.outside
		},
		acquire: func() (motion.Session, func(), error) {
			if f.held != 0 {
				f.t.Fatal("recursive acquire")
			}
			f.phase++
			phase, generation := f.phase, f.generation
			f.log(fmt.Sprintf("ensure %d", phase))
			if f.phase == f.acquireFail {
				return motion.Session{}, nil, errAdapter
			}
			f.held = phase
			f.log(fmt.Sprintf("lock %d generation %d", phase, generation))
			check := func() {
				f.locked()
				if f.held != phase || f.generation != generation {
					f.t.Fatal("stale phase callback")
				}
			}
			return motion.Session{
				MoveRelative: func(dx, dy int32) error {
					check()
					f.log(fmt.Sprintf("move %d generation %d", phase, generation))
					f.x += dx
					f.y += dy
					return nil
				},
				Button: func(b motion.Button, down bool) error {
					check()
					f.log(fmt.Sprintf("button %d %v generation %d", b, down, generation))
					return nil
				},
			}, func() {
				check()
				f.log(fmt.Sprintf("unlock %d", phase))
				f.held = 0
				if phase == 1 {
					f.generation++
					f.log("close between phases")
				}
			}, nil
		},
		setCursor: func(x, y int32) error {
			f.locked()
			if f.held != 2 {
				f.t.Fatal("position in wrong phase")
			}
			f.log(fmt.Sprintf("set %d,%d", x, y))
			if f.setFail {
				return errAdapter
			}
			return nil
		},
		doubleClickTime: func() time.Duration {
			f.locked()
			if f.held != 2 {
				f.t.Fatal("time in wrong phase")
			}
			f.log("dc-time")
			return f.dc
		},
		outside: errBounds, notConverged: errAdapter,
	}
}

var errAdapter = errors.New("adapter effect failure")
var errBounds = errors.New("outside")

func TestAdapterEntranceAndPhaseFailures(t *testing.T) {
	for _, op := range []string{"move", "left", "right", "middle", "double"} {
		for _, stage := range []string{"bounds", "acquire1", "cursor", "acquire2", "success"} {
			t.Run(op+"/"+stage, func(t *testing.T) {
				f := &adapterFixture{t: t}
				switch stage {
				case "bounds":
					f.outside = true
				case "acquire1":
					f.acquireFail = 1
				case "cursor":
					f.cursorFail = true
				case "acquire2":
					f.acquireFail = 2
				}
				m := f.adapter()
				var err error
				switch op {
				case "move":
					err = m.move(0, 0)
				case "left":
					err = m.click(0, 0, motion.Left)
				case "right":
					err = m.click(0, 0, motion.Right)
				case "middle":
					err = m.click(0, 0, motion.Middle)
				case "double":
					err = m.doubleClick(0, 0)
				}
				switch stage {
				case "bounds":
					if err != errBounds || !reflect.DeepEqual(f.trace, []string{"bounds"}) {
						t.Fatal(err, f.trace)
					}
				case "acquire1":
					if err != errAdapter || !reflect.DeepEqual(f.trace, []string{"bounds", "ensure 1"}) {
						t.Fatal(err, f.trace)
					}
				case "cursor":
					if err != errAdapter || f.phase != 1 || f.held != 0 {
						t.Fatal(err, f.trace)
					}
				case "acquire2":
					if op == "move" {
						if err != nil {
							t.Fatal(err)
						}
					} else if err != errAdapter || f.phase != 2 || f.held != 0 {
						t.Fatal(err, f.trace)
					}
				case "success":
					if err != nil || f.held != 0 {
						t.Fatal(err, f.trace)
					}
				}
				if stage != "success" && stage != "acquire2" {
					for _, e := range f.trace {
						if strings.HasPrefix(e, "button") || strings.HasPrefix(e, "ensure 2") {
							t.Fatal("effects after failed move", f.trace)
						}
					}
				}
				if op != "move" && (stage == "success" || stage == "acquire2") {
					s := strings.Join(f.trace, "|")
					if !strings.Contains(s, "unlock 1|close between phases|ensure 2") {
						t.Fatal("lost phase boundary", s)
					}
					if stage == "success" && !strings.Contains(s, "lock 2 generation 1") {
						t.Fatal("not reacquired", s)
					}
				}
			})
		}
	}
}

func TestAdapterOrdinaryButtonTraces(t *testing.T) {
	for _, b := range []motion.Button{motion.Left, motion.Right, motion.Middle} {
		f := &adapterFixture{t: t}
		m := f.adapter()
		if err := m.click(0, 0, b); err != nil {
			t.Fatal(err)
		}
		idx := 0
		for i, e := range f.trace {
			if e == "lock 2 generation 1" {
				idx = i + 1
				break
			}
		}
		want := []string{"sleep 50ms"}
		if b == motion.Left {
			want = append(want, "rand 20", "rand 21", "sleep 30ms")
		}
		want = append(want, fmt.Sprintf("button %d true generation 1", b))
		if b == motion.Left {
			want = append(want, "rand 30", "sleep 75ms")
		} else {
			want = append(want, "rand 41", "sleep 60ms")
		}
		want = append(want, fmt.Sprintf("button %d false generation 1", b), "unlock 2")
		if !reflect.DeepEqual(f.trace[idx:], want) {
			t.Fatalf("%v != %v", f.trace[idx:], want)
		}
	}
}

func TestAdapterDoublePositionAndInterval(t *testing.T) {
	for _, dc := range []time.Duration{0, time.Millisecond, 89 * time.Millisecond, 90 * time.Millisecond, 91 * time.Millisecond, 500 * time.Millisecond} {
		f := &adapterFixture{t: t, dc: dc}
		m := f.adapter()
		if err := m.doubleClick(-5, 7); err != nil {
			t.Fatal(err)
		}
		idx := 0
		for i, e := range f.trace {
			if e == "lock 2 generation 1" {
				idx = i + 1
				break
			}
		}
		// Expected intervals are independent constants, including nanosecond precision.
		wantInterval := map[time.Duration]string{0: "166.666666ms", time.Millisecond: "30ms", 89 * time.Millisecond: "30ms", 90 * time.Millisecond: "30ms", 91 * time.Millisecond: "30.333333ms", 500 * time.Millisecond: "166.666666ms"}[dc]
		want := []string{"set -5,7", "sleep 12ms", "dc-time", "button 0 true generation 1", "sleep 25ms", "button 0 false generation 1", "sleep " + wantInterval, "button 0 true generation 1", "sleep 25ms", "button 0 false generation 1", "unlock 2"}
		if !reflect.DeepEqual(f.trace[idx:], want) {
			t.Fatalf("dc %v: %v != %v", dc, f.trace[idx:], want)
		}
	}
	f := &adapterFixture{t: t, setFail: true}
	m := f.adapter()
	if err := m.doubleClick(0, 0); err != errAdapter {
		t.Fatal(err)
	}
	tail := f.trace[len(f.trace)-2:]
	if !reflect.DeepEqual(tail, []string{"set 0,0", "unlock 2"}) {
		t.Fatal(f.trace)
	}
}
