package motion

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

var (
	errCursor   = errors.New("cursor failure")
	errSend     = errors.New("send first failure")
	errAgain    = errors.New("send retry failure")
	errConverge = errors.New("public convergence failure")
)

type fixture struct {
	trace        []string
	x, y         int32
	rng          *rand.Rand
	reads, sends int
	readFail     int
	sendFails    map[int]error
	sticky       bool
	cursorScript map[int][2]int32
	timer        chan time.Time
	fireAtRead   int
	fired        bool
}

func newFixture(seed int64) *fixture {
	return &fixture{rng: rand.New(rand.NewSource(seed)), sendFails: map[int]error{}, cursorScript: map[int][2]int32{}}
}
func (f *fixture) log(format string, a ...any) { f.trace = append(f.trace, fmt.Sprintf(format, a...)) }
func (f *fixture) runtime() Runtime {
	return Runtime{
		Cursor: func() (int32, int32, error) {
			f.reads++
			if v, ok := f.cursorScript[f.reads]; ok {
				f.x, f.y = v[0], v[1]
			}
			f.log("cursor %d,%d", f.x, f.y)
			if f.timer != nil && !f.fired && f.fireAtRead == f.reads {
				close(f.timer)
				f.fired = true
			}
			if f.reads == f.readFail {
				return 0, 0, errCursor
			}
			return f.x, f.y, nil
		},
		Sleep: func(d time.Duration) { f.log("sleep %s", d) },
		Intn:  func(n int) int { v := f.rng.Intn(n); f.log("rand %d=%d", n, v); return v },
		After: func(d time.Duration) <-chan time.Time {
			f.log("after %s", d)
			f.timer = make(chan time.Time)
			if f.fireAtRead < 0 {
				close(f.timer)
				f.fired = true
			}
			return f.timer
		},
	}
}
func (f *fixture) session() Session {
	return Session{
		MoveRelative: func(dx, dy int32) error {
			f.sends++
			f.log("move %d,%d", dx, dy)
			if err := f.sendFails[f.sends]; err != nil {
				return err
			}
			if !f.sticky {
				f.x += dx
				f.y += dy
			}
			return nil
		},
		Button: func(b Button, down bool) error {
			f.sends++
			f.log("button %d %v", b, down)
			return f.sendFails[f.sends]
		},
	}
}
func sameResult(t *testing.T, a, b *fixture, ae, be error) {
	t.Helper()
	if (ae == nil) != (be == nil) || ae != nil && ae.Error() != be.Error() {
		t.Fatalf("errors: %v vs %v", ae, be)
	}
	if !reflect.DeepEqual(a.trace, b.trace) {
		t.Fatalf("trace differs\nnew: %v\nold: %v", a.trace, b.trace)
	}
}

// The expected trace comes from the frozen pre-extraction implementation, not
// the new implementation. Fakes vary random draws, lag, read/send failures and
// timer delivery; comparison includes every observation and attempted send.
func TestMoveFrozenBaselineTrace(t *testing.T) {
	for _, distance := range []int32{0, 1, 4, 24, 25, 99, 100, 499, 500, 999, 1000, 2001, -99, -100, -500, -1000} {
		for _, seed := range []int64{0, 1, 19} {
			for scenario := 0; scenario < 102; scenario++ {
				t.Run(fmt.Sprintf("d%d_seed%d_case%d", distance, seed, scenario), func(t *testing.T) {
					a, b := newFixture(seed), newFixture(seed)
					for _, f := range []*fixture{a, b} {
						f.x, f.y = -71, 29
						switch {
						case scenario == 1:
							f.sticky = true
						case scenario == 2:
							f.fireAtRead = -1
						case scenario == 3:
							f.fireAtRead = 4
						case scenario < 53 && scenario >= 4:
							f.readFail = scenario - 3
						case scenario >= 53:
							f.sendFails[scenario-52] = errSend
						}
					}
					ae := Move(a.session(), a.runtime(), distance-71, 29-distance/3, errConverge)
					be := legacyMove(b.session(), b.runtime(), distance-71, 29-distance/3, errConverge)
					sameResult(t, a, b, ae, be)
				})
			}
		}
	}
}

func TestButtonsFrozenBaselineTrace(t *testing.T) {
	for _, b := range []Button{Left, Right, Middle} {
		for seed := int64(0); seed < 50; seed++ {
			for _, fail := range []int{0, 1, 2, 3} {
				a, c := newFixture(seed), newFixture(seed)
				for _, f := range []*fixture{a, c} {
					if fail > 0 {
						f.sendFails[fail] = errSend
					}
					if fail == 2 {
						f.sendFails[3] = errAgain
					}
				}
				sameResult(t, a, c, Click(a.session(), a.runtime(), b), legacyClick(c.session(), c.runtime(), b))
			}
		}
	}
	for _, interval := range []time.Duration{30 * time.Millisecond, 500 * time.Millisecond / 3} {
		for fail := 0; fail <= 4; fail++ {
			for _, retryFails := range []bool{false, true} {
				a, b := newFixture(0), newFixture(0)
				for _, f := range []*fixture{a, b} {
					if fail > 0 {
						f.sendFails[fail] = errSend
						if retryFails {
							f.sendFails[fail+1] = errAgain
						}
					}
				}
				sameResult(t, a, b, DoubleClick(a.session(), a.runtime(), interval), legacyDouble(b.session(), b.runtime(), interval))
			}
		}
	}
}

func count(trace []string, prefix string) int {
	n := 0
	for _, e := range trace {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}
func TestMoveTimingAndTolerance(t *testing.T) {
	// At the target still reads initial position, creates a timer, visits all five
	// trajectory observations, then waits 20ms before the convergence read.
	f := newFixture(0)
	if err := Move(f.session(), f.runtime(), 0, 0, errConverge); err != nil {
		t.Fatal(err)
	}
	want := []string{"cursor 0,0", "after 3s", "cursor 0,0", "cursor 0,0", "cursor 0,0", "cursor 0,0", "cursor 0,0", "sleep 20ms", "cursor 0,0"}
	if !reflect.DeepEqual(f.trace, want) {
		t.Fatalf("zero move trace %v", f.trace)
	}
	for _, tc := range []struct {
		dist  int32
		steps int
		delay string
	}{{99, 19, "sleep 5ms"}, {100, 20, "sleep 5ms"}, {499, 20, "sleep 5ms"}, {500, 30, "sleep 5ms"}, {999, 30, "sleep 5ms"}, {1000, 40, "sleep 3ms"}} {
		f := newFixture(3)
		f.sticky = true
		_ = Move(f.session(), f.runtime(), tc.dist, 0, errConverge)
		if f.reads != tc.steps+7 {
			t.Fatalf("distance %d reads %d", tc.dist, f.reads)
		}
		if count(f.trace, tc.delay) == 0 || count(f.trace, "sleep 20ms") != 5 {
			t.Fatalf("distance %d timing %v", tc.dist, f.trace)
		}
		// Fifth final send is followed by exactly one immediate read, not a wait.
		if !strings.HasPrefix(f.trace[len(f.trace)-2], "move ") || !strings.HasPrefix(f.trace[len(f.trace)-1], "cursor ") {
			t.Fatal(f.trace)
		}
	}
	for _, delta := range []int32{2, 3, -2, -3} {
		f := newFixture(0)
		f.sticky = true
		f.cursorScript[7] = [2]int32{delta, 0}
		err := Move(f.session(), f.runtime(), 0, 0, errConverge)
		if delta == 2 || delta == -2 {
			if err != nil || f.sends != 0 {
				t.Fatal(delta, err, f.trace)
			}
		} else if err != errConverge || f.sends != 5 {
			t.Fatal(delta, err, f.trace)
		}
	}
	// A timeout delivered at the first convergence observation is not checked:
	// it is a trajectory timer, not a whole-operation deadline.
	f = newFixture(0)
	f.fireAtRead = 7
	if err := Move(f.session(), f.runtime(), 0, 0, errConverge); err != nil {
		t.Fatal(err)
	}
	// The fifth correction can finally become visible on the immediate read.
	f = newFixture(0)
	f.sticky = true
	f.cursorScript[7] = [2]int32{9, 0}
	f.cursorScript[12] = [2]int32{0, 0}
	if err := Move(f.session(), f.runtime(), 0, 0, errConverge); err != nil || f.sends != 5 {
		t.Fatal(err, f.trace)
	}
}

func TestButtonFailureBoundaries(t *testing.T) {
	for _, b := range []Button{Left, Right, Middle} {
		f := newFixture(0)
		f.sendFails[1] = errSend
		if err := Click(f.session(), f.runtime(), b); err != errSend || f.sends != 1 {
			t.Fatal(err, f.trace)
		}
		f = newFixture(0)
		f.sendFails[2] = errSend
		f.sendFails[3] = errAgain
		if err := Click(f.session(), f.runtime(), b); err != errSend || f.sends != 3 || f.trace[len(f.trace)-2] != "sleep 6ms" {
			t.Fatal(err, f.trace)
		}
	}
	for fail := 1; fail <= 4; fail++ {
		f := newFixture(0)
		f.sendFails[fail] = errSend
		f.sendFails[fail+1] = errAgain
		if err := DoubleClick(f.session(), f.runtime(), time.Second/6); err != errSend || f.sends != fail+1 || f.trace[len(f.trace)-2] != "sleep 6ms" {
			t.Fatal(err, f.trace)
		}
	}
}

func TestMoveStickyFailureIdentityAndFrozenTrace(t *testing.T) {
	// A non-updating cursor reaches every final correction and the post-fifth
	// read. Sweep failures there as well as the initial/trajectory observations.
	for _, readFailure := range []bool{true, false} {
		for at := 1; at <= 47; at++ {
			a, b := newFixture(9), newFixture(9)
			for _, f := range []*fixture{a, b} {
				f.sticky = true
				if readFailure {
					f.readFail = at
				} else {
					f.sendFails[at] = errSend
				}
			}
			ae := Move(a.session(), a.runtime(), 1000, -333, errConverge)
			be := legacyMove(b.session(), b.runtime(), 1000, -333, errConverge)
			sameResult(t, a, b, ae, be)
			if be == errCursor && ae != errCursor || be == errSend && ae != errSend || be == errConverge && ae != errConverge {
				t.Fatal("error identity changed", ae, be)
			}
		}
	}
}

func TestOrdinaryRandomEndpoints(t *testing.T) {
	for _, upper := range []bool{false, true} {
		for _, b := range []Button{Left, Right, Middle} {
			var trace []string
			r := Runtime{
				Sleep: func(d time.Duration) { trace = append(trace, "sleep "+d.String()) },
				Intn: func(n int) int {
					trace = append(trace, fmt.Sprintf("rand %d", n))
					if upper {
						return n - 1
					}
					return 0
				},
			}
			s := Session{Button: func(b Button, down bool) error {
				trace = append(trace, fmt.Sprintf("button %d %v", b, down))
				return nil
			}}
			if err := Click(s, r, b); err != nil {
				t.Fatal(err)
			}
			want := []string{"sleep 50ms"}
			if b == Left {
				if upper {
					want = append(want, "rand 20", "rand 27", "sleep 52ms", "button 0 true", "rand 30", "sleep 89ms", "button 0 false")
				} else {
					want = append(want, "rand 20", "rand 13", "sleep 14ms", "button 0 true", "rand 30", "sleep 60ms", "button 0 false")
				}
			} else {
				hold := "40ms"
				if upper {
					hold = "80ms"
				}
				want = append(want, fmt.Sprintf("button %d true", b), "rand 41", "sleep "+hold, fmt.Sprintf("button %d false", b))
			}
			if !reflect.DeepEqual(trace, want) {
				t.Fatalf("%v != %v", trace, want)
			}
		}
	}
}

// Moved unchanged from hid/control_test.go with the extracted helper.
func TestHumanArrived(t *testing.T) {
	cases := []struct {
		dx, dy int32
		want   bool
	}{
		{2, 0, true},
		{2, 2, true},
		{1, 1, true},
		{0, 0, true},
		{3, 0, false},
		{0, 3, false},
		{-2, 2, true},
		{-3, 0, false},
	}
	for _, tc := range cases {
		if got := humanArrived(tc.dx, tc.dy); got != tc.want {
			t.Errorf("humanArrived(%d, %d) = %v, want %v", tc.dx, tc.dy, got, tc.want)
		}
	}
}

func TestMoveJitterZeroContinuesWithoutSendOrSleep(t *testing.T) {
	a, b := newFixture(0), newFixture(0)
	for _, f := range []*fixture{a, b} {
		f.cursorScript[2] = [2]int32{5, 0}
	}
	runtimeFor := func(f *fixture) Runtime {
		r := f.runtime()
		r.Intn = func(n int) int { f.log("rand %d=%d", n, n/2); return n / 2 }
		return r
	}
	ae := Move(a.session(), runtimeFor(a), 100, 0, errConverge)
	be := legacyMove(b.session(), runtimeFor(b), 100, 0, errConverge)
	sameResult(t, a, b, ae, be)
	want := []string{"cursor 0,0", "after 3s", "cursor 5,0", "rand 3=1", "rand 3=1", "cursor 5,0"}
	if !reflect.DeepEqual(a.trace[:len(want)], want) {
		t.Fatal(a.trace)
	}
}

func TestHumanSleepZeroBasePreservesClamp(t *testing.T) {
	var got time.Duration
	r := Runtime{Intn: func(n int) int {
		if n != 3 {
			t.Fatal(n)
		}
		return 0
	}, Sleep: func(d time.Duration) { got = d }}
	humanSleep(r, 0)
	if got != 0 {
		t.Fatal(got)
	}
}
