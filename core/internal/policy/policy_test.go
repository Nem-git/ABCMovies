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
	if d[KeyBandwidth] != DefaultBandwidthValue || d[KeyEncode] != DefaultEncodeValue {
		t.Fatalf("defaults = %v, want %s bandwidth / encode %s", d, DefaultBandwidthValue, DefaultEncodeValue)
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
	if s[KeyBandwidth] != DefaultBandwidthValue {
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
		override map[string]string
		cap      int // 0 = no cap declared
		want     int
	}{
		{nil, 0, 3},                          // default vs none
		{m(KeyConcurrentStreams, "4"), 0, 4}, // override lifts, no cap
		{nil, 2, 2},                          // cap pulls down
		{m(KeyConcurrentStreams, "5"), 2, 2},
		{m(KeyConcurrentStreams, "5"), 9, 5},
	} {
		got := EffectiveAccountLimit(d.With(Set(tc.override)), CapsFor(uint32(tc.cap)))
		if got != tc.want {
			t.Fatalf("override=%v cap=%d: want %d, got %d", tc.override, tc.cap, tc.want, got)
		}
	}
}

// A cap parses on its own, apart from the policy vocabulary: it knows one
// key, defaults to nothing, and refuses a policy key smuggled into it —
// otherwise "cap" and "policy" would collapse back into one map.
func TestParseCapsIsItsOwnVocabulary(t *testing.T) {
	c, err := ParseCaps(map[string]string{KeyConcurrentStreams: "2"})
	if err != nil {
		t.Fatalf("ParseCaps: %v", err)
	}
	if n, ok := c.Streams(); !ok || n != 2 {
		t.Fatalf("streams = %d, present %v; want 2, true", n, ok)
	}
	// Absent is the unbound answer, not zero-admit.
	empty, err := ParseCaps(nil)
	if err != nil {
		t.Fatalf("ParseCaps(nil): %v", err)
	}
	if _, ok := empty.Streams(); ok {
		t.Fatal("nil caps declared a ceiling; want unbound")
	}
	for _, bad := range []map[string]string{
		{KeyConcurrentStreams: "0"},
		{KeyConcurrentStreams: "many"},
		{KeyBandwidth: "unlimited"}, // a policy key is not a cap
	} {
		if _, err := ParseCaps(bad); err == nil {
			t.Errorf("ParseCaps(%v): want error, got nil", bad)
		}
	}
}

// CapsFor's zero is the documented "unbound" spelling, and the recorded
// round-trip renders exactly what was declared — an unbound account records
// nothing rather than a cap of zero.
func TestCapsRoundTripThroughLimits(t *testing.T) {
	if _, ok := CapsFor(0).Streams(); ok {
		t.Fatal("CapsFor(0) declared a ceiling; want unbound")
	}
	if got := CapsFor(0).Limits(); got != nil {
		t.Fatalf("unbound Limits() = %v, want nil", got)
	}
	c := CapsFor(4)
	back, err := ParseCaps(c.Limits())
	if err != nil {
		t.Fatalf("round trip: %v", err)
	}
	if n, ok := back.Streams(); !ok || n != 4 {
		t.Fatalf("round trip streams = %d, present %v; want 4, true", n, ok)
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
