package registry_test

import (
	"testing"

	"github.com/nem-git/abcmovies/internal/registry"
	"github.com/nem-git/abcmovies/internal/providers/stub"
)

func TestNewEmpty(t *testing.T) {
	r := registry.New()
	if got := len(r.All()); got != 0 {
		t.Errorf("All() returned %d providers, want 0", got)
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{Tag: "test"})

	if err := r.Register(p); err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	got, err := r.Get("test")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Tag() != "test" {
		t.Errorf("Tag() = %q, want %q", got.Tag(), "test")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{Tag: "dup"})

	if err := r.Register(p); err != nil {
		t.Fatalf("first Register() error: %v", err)
	}
	if err := r.Register(p); err != registry.ErrDuplicateTag {
		t.Errorf("second Register() error = %v, want ErrDuplicateTag", err)
	}
}

func TestGetNotFound(t *testing.T) {
	r := registry.New()
	_, err := r.Get("nonexistent")
	if err != registry.ErrNotFound {
		t.Errorf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestAll(t *testing.T) {
	r := registry.New()
	p1 := stub.New(stub.Config{Tag: "a"})
	p2 := stub.New(stub.Config{Tag: "b"})

	r.Register(p1)
	r.Register(p2)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d providers, want 2", len(all))
	}

	tags := make(map[string]bool)
	for _, p := range all {
		tags[p.Tag()] = true
	}
	if !tags["a"] || !tags["b"] {
		t.Errorf("All() missing providers: got tags %v", tags)
	}
}

