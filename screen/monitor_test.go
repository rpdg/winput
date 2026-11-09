package screen

import "testing"

func TestMonitorsReusesOneCallback(t *testing.T) {
	for i := 0; i < 2100; i++ {
		monitors, err := Monitors()
		if err != nil {
			t.Fatal(err)
		}
		if len(monitors) == 0 {
			t.Fatal("EnumDisplayMonitors returned no monitors")
		}
	}
}
