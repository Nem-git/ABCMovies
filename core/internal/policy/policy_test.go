package policy

import (
	"strings"
	"testing"
)

func TestDefaultsHaveTheShippedBaseline(t *testing.T) {
	d := Defaults()
	if s, ok := d.Streams(); !ok || s != 3 {
		t.Fatalf("default streams = %d, present %v; want 3, true", s, ok)
	}
	if d[KeyBandwidth] != "unlimited" || d[KeyEncode] != "disabled" {
		t.Fatalf("defaults = %v, want unlimited bandwidth / encode disabled", d)
	}
}

func TestParseInstanceUnknownKeyIsAStartupError(t *testing.T) {
	if _, err := ParseInstance(map[string]string{"concurrentStream": "4"}); err == nil {
		t.Fatal("unknown key: want error, got nil")
	} else if !strings.Contains(err.Error(), "concurrentStream") {
		t.Fatalf("error should name the unknown key, got %v", err)
	}
}

func TestParseInstanceValueValidation(t *testing.T) {
	for _, bad := range []map[string]string{
		{KeyConcurrentStreams: "0"},
		{KeyConcurrentStreams: "many"},
		{KeyBandwidth: "often"},
		{KeyBandwidth: "10GiB"},
		{KeyEncode: "yes"},
		{KeyTimeWindow: "  "},
	} {
		if _, err := ParseInstance(bad); err == nil {
			t.Fatalf("ParseInstance(%v): want error, got nil", bad)
		}
	}
}

func TestParseInstanceMergesOverDefaults(t *testing.T) {
	s, err := ParseInstance(map[string]string{KeyConcurrentStreams: "5"})
	if err != nil {
		t.Fatalf("ParseInstance: %v", err)
	}
	if n, _ := s.Streams(); n != 5 {
		t.Fatalf("streams = %d, want 5", n)
	}
	if s[KeyBandwidth] != "unlimited" {
		t.Fatalf("bandwidth fell off the defaults: %v", s)
	}
}

func TestWithIsNonDestructive(t *testing.T) {
	a := Defaults()
	b := a.With(Set{KeyConcurrentStreams: "7"})
	if n, _ := a.Streams(); n != 3 {
		t.Fatalf("With mutated the receiver: %d", n)
	}
	if n, _ := b.Streams(); n != 7 {
		t.Fatalf("merged streams = %d, want 7", n)
	}
}

func TestEffectiveAccountLimitIsPolicyCappedByProviderCap(t *testing.T) {
	d := Defaults() // streams 3
	for _, tc := range []struct {
		override, cap map[string]string
		want          int
	}{
		{nil, nil, 3},                          // default vs none
		{m(KeyConcurrentStreams, "4"), nil, 4}, // override lifts, no cap
		{nil, m(KeyConcurrentStreams, "2"), 2}, // cap pulls down
		{m(KeyConcurrentStreams, "5"), m(KeyConcurrentStreams, "2"), 2},
		{m(KeyConcurrentStreams, "5"), m(KeyConcurrentStreams, "9"), 5},
	} {
		got := EffectiveAccountLimit(d.With(Set(tc.override)), Set(tc.cap))
		if got != tc.want {
			t.Fatalf("override=%v cap=%v: want %d, got %d", tc.override, tc.cap, tc.want, got)
		}
	}
}

func TestParseOverlayAllowsEmptyAndRejectsUnknown(t *testing.T) {
	if s, err := ParseOverlay(nil); err != nil || s == nil {
		t.Fatalf("nil overlay: want empty set, nil error; got %v, %v", s, err)
	}
	if _, err := ParseOverlay(map[string]string{"streams": "2"}); err == nil {
		t.Fatal("unknown overlay key: want error")
	}
}

func m(k, v string) map[string]string { return map[string]string{k: v} }
