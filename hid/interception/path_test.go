package interception

import (
	"sync"
	"testing"
)

func TestSetLibraryPathSynchronizedWithLoad(t *testing.T) {
	const missing = `C:\no\such\winput-interception.dll`
	SetLibraryPath(missing)
	t.Cleanup(func() { SetLibraryPath("interception.dll") })
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			SetLibraryPath(missing)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			if err := Load(); err == nil {
				t.Errorf("Load() = nil, want failure for a missing library")
			}
		}
	}()
	wg.Wait()
}
