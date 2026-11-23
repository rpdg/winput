package motion

import "time"

func humanSleep(r Runtime, base int) {
	maxJitter := base / 3
	if maxJitter == 0 {
		maxJitter = 1
	}
	jitter := r.Intn(2*maxJitter+1) - maxJitter
	duration := base + jitter
	if duration < 0 {
		duration = 0
	}
	r.Sleep(time.Duration(duration) * time.Millisecond)
}

// retryButton retries the same stroke once, preserving its first error.
func retryButton(s Session, r Runtime, b Button, down bool) error {
	if err := s.Button(b, down); err != nil {
		r.Sleep(6 * time.Millisecond)
		if err2 := s.Button(b, down); err2 != nil {
			return err
		}
	}
	return nil
}

// Click is only the ordinary button phase, after a separate completed Move.
func Click(s Session, r Runtime, b Button) error {
	r.Sleep(50 * time.Millisecond)
	if b == Left {
		humanSleep(r, 20+r.Intn(20))
	}
	if err := s.Button(b, true); err != nil {
		return err
	}
	if b == Left {
		r.Sleep(time.Duration(60+r.Intn(30)) * time.Millisecond)
	} else {
		humanSleep(r, 60)
	}
	return retryButton(s, r, b, false)
}

// DoubleClick is only the button sequence. The adapter has already positioned
// the system cursor, waited 12ms and read/calculated the system interval.
func DoubleClick(s Session, r Runtime, interval time.Duration) error {
	if err := retryButton(s, r, Left, true); err != nil {
		return err
	}
	r.Sleep(25 * time.Millisecond)
	if err := retryButton(s, r, Left, false); err != nil {
		return err
	}
	r.Sleep(interval)
	if err := retryButton(s, r, Left, true); err != nil {
		return err
	}
	r.Sleep(25 * time.Millisecond)
	return retryButton(s, r, Left, false)
}
