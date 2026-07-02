package wework

import "testing"

func TestEnabled(t *testing.T) {
	// tagID 可选,不参与启用判断
	if !New("corp", "agent", "secret", "").Enabled() {
		t.Fatal("without tagID must still be enabled")
	}
	for _, c := range []struct{ corp, agent, secret string }{
		{"", "agent", "secret"},
		{"corp", "", "secret"},
		{"corp", "agent", ""},
	} {
		if New(c.corp, c.agent, c.secret, "tag").Enabled() {
			t.Fatalf("missing field must be disabled: %+v", c)
		}
	}
}
