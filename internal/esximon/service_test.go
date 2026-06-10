package esximon

import (
	"encoding/json"
	"testing"
)

func TestStickyUSBJSONKeepsPreviousControllers(t *testing.T) {
	prev := USBState{
		Controllers: []USBController{{
			PCIAddr: "0000:00:14.0",
			Name:    "Intel USB xHCI Host Controller",
		}},
		ArbitratorRunning: true,
		AvailableForPassthrough: []USBPassthroughDevice{{
			Bus: 1, Dev: 2, VID: "old", PID: "dev", Name: "old device", Enabled: true,
		}},
	}
	prevJSON, err := json.Marshal(prev)
	if err != nil {
		t.Fatal(err)
	}

	cur := USBState{
		ArbitratorRunning: true,
		AvailableForPassthrough: []USBPassthroughDevice{{
			Bus: 1, Dev: 3, VID: "152d", PID: "a576", Name: "JMicron", Enabled: true,
		}},
		arbitratorKnown:  true,
		passthroughKnown: true,
	}
	gotJSON := stickyUSBJSON(cur, string(prevJSON))

	var got USBState
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Controllers) != 1 || got.Controllers[0].PCIAddr != "0000:00:14.0" {
		t.Fatalf("controllers were not preserved: %#v", got.Controllers)
	}
	if len(got.AvailableForPassthrough) != 1 || got.AvailableForPassthrough[0].PID != "a576" {
		t.Fatalf("passthrough devices were not refreshed: %#v", got.AvailableForPassthrough)
	}
}

func TestStickyUSBJSONClearsKnownEmptyPassthrough(t *testing.T) {
	prev := USBState{
		Controllers: []USBController{{PCIAddr: "0000:00:14.0", Name: "Intel USB"}},
		AvailableForPassthrough: []USBPassthroughDevice{{
			Bus: 1, Dev: 2, VID: "old", PID: "dev", Name: "old device", Enabled: true,
		}},
	}
	prevJSON, err := json.Marshal(prev)
	if err != nil {
		t.Fatal(err)
	}

	cur := USBState{
		Controllers:      prev.Controllers,
		controllersKnown: true,
		passthroughKnown: true,
	}
	gotJSON := stickyUSBJSON(cur, string(prevJSON))

	var got USBState
	if err := json.Unmarshal([]byte(gotJSON), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Controllers) != 1 {
		t.Fatalf("controllers = %#v", got.Controllers)
	}
	if len(got.AvailableForPassthrough) != 0 {
		t.Fatalf("known empty passthrough should clear previous values: %#v", got.AvailableForPassthrough)
	}
}
