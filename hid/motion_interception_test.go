package hid

// Windows compilation-only in the Linux delivery. All effects here are fakes;
// selecting these tests on Windows never calls Init, loads DLLs or sends input.
import (
	"errors"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/rpdg/winput/hid/interception"
	"github.com/rpdg/winput/internal/motion"
)

func TestMotionSessionCapturedHandlesAndStrokes(t *testing.T) {
	type sent struct {
		c interception.Context
		d interception.Device
		s interception.MouseStroke
	}
	var got []sent
	failure := errors.New("raw send failure")
	send := func(c interception.Context, d interception.Device, s *interception.MouseStroke) error {
		got = append(got, sent{c, d, *s})
		return failure
	}
	s := mouseSession(17, 4, send)
	// Mutating lifecycle globals must not change the handles captured by this phase.
	initMutex.Lock()
	oldC, oldD := ctx, mouseDev
	ctx, mouseDev = 99, 8
	initMutex.Unlock()
	defer func() { initMutex.Lock(); ctx, mouseDev = oldC, oldD; initMutex.Unlock() }()
	if err := s.MoveRelative(-123, 456); err != failure {
		t.Fatal(err)
	}
	for _, b := range []motion.Button{motion.Left, motion.Right, motion.Middle} {
		for _, down := range []bool{true, false} {
			if err := s.Button(b, down); err != failure {
				t.Fatal(err)
			}
		}
	}
	want := []sent{{17, 4, interception.MouseStroke{Flags: interception.MouseFlagMoveRelative, X: -123, Y: 456}}}
	for _, state := range []uint16{interception.MouseStateLeftDown, interception.MouseStateLeftUp, interception.MouseStateRightDown, interception.MouseStateRightUp, interception.MouseStateMiddleDown, interception.MouseStateMiddleUp} {
		want = append(want, sent{17, 4, interception.MouseStroke{State: state}})
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v != %+v", got, want)
	}
}

// Uses the actual initMutex and actual mouseSession assembly, replacing only
// the raw DLL send. TryRLock failing while our reader is held proves a writer
// has queued, rather than guessing its timing with Sleep.
func TestMotionSessionPendingWriterNoRecursiveReadLock(t *testing.T) {
	var wg sync.WaitGroup
	initMutex.RLock()
	var once sync.Once
	release := func() { once.Do(initMutex.RUnlock) }
	defer release()
	writerDone := make(chan struct{})
	wg.Add(1)
	go func() { defer wg.Done(); initMutex.Lock(); initMutex.Unlock(); close(writerDone) }()
	deadline := time.Now().Add(2 * time.Second)
	for initMutex.TryRLock() {
		initMutex.RUnlock()
		if time.Now().After(deadline) {
			release()
			wg.Wait()
			t.Fatal("writer never queued")
		}
		runtime.Gosched()
	}
	s := mouseSession(17, 4, func(c interception.Context, d interception.Device, st *interception.MouseStroke) error { return nil })
	done := make(chan error, 1)
	go func() {
		if err := s.MoveRelative(1, -2); err != nil {
			done <- err
			return
		}
		done <- s.Button(motion.Left, true)
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(2 * time.Second):
		// Releasing the outer reader lets an accidental recursive read drain too.
		release()
		<-writerDone
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("callback stuck after reader release")
		}
		t.Error("session callback blocked behind pending writer")
	}
	release()
	wg.Wait()
}
