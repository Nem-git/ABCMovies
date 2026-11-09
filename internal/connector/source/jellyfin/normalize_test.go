package jellyfin

import "testing"

func TestNormalizeEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"default scheme", "media.example.test", "https://media.example.test"},
		{"lowercase scheme", "HTTP://Media.Example.Test/", "http://media.example.test"},
		{"https default port dropped", "https://h:443/", "https://h"},
		{"http default port dropped", "http://h:80/", "http://h"},
		{"https non-default kept", "https://h:8096/", "https://h:8096"},
		{"http non-default kept", "http://h:8096/", "http://h:8096"},
		{"port zero rejected", "https://h:0", ""},
		{"port 65536 rejected", "https://h:65536", ""},
		{"port not numeric", "https://h:abc", ""},
		{"userinfo rejected", "https://u:p@media.example.test/", "ERR"},
		{"user only rejected", "https://u@media.example.test/", "ERR"},
		{"ipv4", "http://127.0.0.1:8096/", "http://127.0.0.1:8096"},
		{"query stripped", "https://media.example.test/?x=1", "https://media.example.test"},
		{"fragment stripped", "https://media.example.test/#x", "https://media.example.test"},
		{"nested path", "https://media.example.test/jellyfin/", "https://media.example.test/jellyfin"},
		{"trailing slash root", "https://media.example.test/", "https://media.example.test"},
		{"non-http scheme", "ftp://media.example.test", ""},
		{"empty", "   ", ""},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeEndpoint(tc.in)
			if tc.want == "ERR" {
				if err == nil {
					t.Fatalf("normalizeEndpoint(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if tc.want == "" {
				if err == nil {
					t.Fatalf("normalizeEndpoint(%q) = %q, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeEndpoint(%q) err = %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeEndpoint(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
