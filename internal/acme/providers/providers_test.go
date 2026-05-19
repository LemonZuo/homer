package acmeproviders

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateUnregisteredProvider(t *testing.T) {
	err := Validate("definitely-not-a-provider", map[string]string{})
	if !errors.Is(err, ErrNoValidator) {
		t.Fatalf("unregistered provider should return ErrNoValidator, got %v", err)
	}
}

func TestTrimSDKErr(t *testing.T) {
	t.Run("cuts at first newline", func(t *testing.T) {
		got := trimSDKErr(errors.New("main reason\nRequestId: abc\nstack..."))
		if got != "main reason" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("short single line unchanged", func(t *testing.T) {
		if got := trimSDKErr(errors.New("boom")); got != "boom" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("truncates over 240 chars", func(t *testing.T) {
		long := strings.Repeat("x", 300)
		got := trimSDKErr(errors.New(long))
		if len(got) != 243 || !strings.HasSuffix(got, "...") {
			t.Fatalf("len=%d suffix=%q", len(got), got[len(got)-3:])
		}
	})
}
