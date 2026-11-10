package service

import "testing"

func TestLive(t *testing.T) {
	if got := Live(); got != "ok" {
		t.Fatalf("Live() = %q, want %q", got, "ok")
	}
}
