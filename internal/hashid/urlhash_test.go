package hashid

import (
	"regexp"
	"testing"
)

var hashPattern = regexp.MustCompile(`^[a-f0-9]{12}$`)

func TestURLHashDeterministic(t *testing.T) {
	const url = "https://cdn.example.com/keys/enc.key?token=abc&expires=9999999999"
	first := URLHash(url)
	for i := 0; i < 10; i++ {
		if got := URLHash(url); got != first {
			t.Fatalf("URLHash(%q) not deterministic: %q then %q", url, first, got)
		}
	}
}

func TestURLHashFormat(t *testing.T) {
	got := URLHash("https://cdn.example.com/movie/master.m3u8")
	if !hashPattern.MatchString(got) {
		t.Errorf("URLHash() = %q, want 12 lowercase hex characters", got)
	}
}

func TestURLHashDistinct(t *testing.T) {
	cases := [][2]string{
		{"https://cdn.example.com/keys/key.php?r=52", "https://cdn.example.com/keys/key.php?r=53"},
		{"https://cdn.example.com/a.m3u8", "https://cdn.example.com/b.m3u8"},
		{"https://cdn.example.com/movie/keys/enc.key", "https://cdn.example.com/movie/keys/enc.key?token=x"},
		{"https://cdn.example.com/720p/variant.m3u8", "http://cdn.example.com/720p/variant.m3u8"},
	}
	for _, c := range cases {
		if URLHash(c[0]) == URLHash(c[1]) {
			t.Errorf("URLHash(%q) == URLHash(%q) == %q, want distinct", c[0], c[1], URLHash(c[0]))
		}
	}
}
