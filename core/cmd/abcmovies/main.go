package main

import (
	"fmt"
	"os"

	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/registry"
)

func main() {
	r := registry.New()
	defer r.Close()

	caps, err := r.Admit("builtin", builtin.New())
	if err != nil {
		fmt.Fprintf(os.Stderr, "abcmovies: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("registry up; built-in slot admitted:")
	for _, c := range caps {
		fmt.Printf("  %s v%d\n", c.Name, c.Version)
	}
}
