package acme

import "testing"

func TestMustJSON(t *testing.T) {
	if got := MustJSON(map[string]int{"a": 1}); got != "{\n  \"a\": 1\n}" {
		t.Fatalf("got %q", got)
	}
	// 不可序列化的值回退 "{}"。
	if got := MustJSON(make(chan int)); got != "{}" {
		t.Fatalf("unserializable should fall back to {}, got %q", got)
	}
}

func TestEmptyJSON(t *testing.T) {
	if got := EmptyJSON("   "); got != "{}" {
		t.Fatalf("blank => %q want {}", got)
	}
	if got := EmptyJSON(`{"k":1}`); got != `{"k":1}` {
		t.Fatalf("non-blank passthrough failed: %q", got)
	}
}
