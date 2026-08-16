package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/registry"
)

type suite struct {
	Contract  string `json:"contract"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Slot      string `json:"slot"`
	Transport string `json:"transport"`
}

type capability struct {
	Name    string `json:"name"`
	Version uint32 `json:"version"`
}

type expected struct {
	Admitted     bool         `json:"admitted"`
	Capabilities []capability `json:"capabilities"`
}

type fixture struct {
	Name        string       `json:"name"`
	Request     string       `json:"request"`
	Declaration []capability `json:"declaration"`
	Expected    expected     `json:"expected"`
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: fixture-runner <fixtures-root>")
		os.Exit(2)
	}
	root := os.Args[1]

	suites := []string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "suite.json" {
			suites = append(suites, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "fixture-runner: %v\n", err)
		os.Exit(1)
	}
	if len(suites) == 0 {
		fmt.Fprintf(os.Stderr, "fixture-runner: no suites found under %s\n", root)
		os.Exit(1)
	}

	total, failed := 0, 0
	for _, suitePath := range suites {
		s, err := readSuite(suitePath)
		if err != nil {
			fmt.Printf("FAIL %s: %v\n", suitePath, err)
			failed++
			continue
		}
		cases, err := readCases(filepath.Dir(suitePath))
		if err != nil {
			fmt.Printf("FAIL %s: %v\n", suitePath, err)
			failed++
			continue
		}
		for _, c := range cases {
			total++
			if err := runCase(s, c); err != nil {
				failed++
				fmt.Printf("FAIL %s %s: %v\n", suitePath, c.Name, err)
			} else {
				fmt.Printf("PASS %s %s\n", suitePath, c.Name)
			}
		}
	}

	fmt.Printf("fixtures: %d run, %d passed, %d failed\n", total, total-failed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func readSuite(path string) (suite, error) {
	var s suite
	b, err := os.ReadFile(path)
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse suite: %w", err)
	}
	return s, nil
}

func readCases(dir string) ([]fixture, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "cases", "*.json"))
	if err != nil {
		return nil, err
	}
	cases := make([]fixture, 0, len(paths))
	for _, p := range paths {
		var c fixture
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &c); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		cases = append(cases, c)
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].Name < cases[j].Name })
	return cases, nil
}

func runCase(s suite, c fixture) error {
	reg := registry.New()
	defer reg.Close()

	if c.Expected.Admitted {
		if s.Kind != "handshake" {
			return fmt.Errorf("kind %q: admission expected only by handshake fixtures", s.Kind)
		}
		caps, err := reg.Admit(c.Name, builtin.New())
		if err != nil {
			return fmt.Errorf("handshake: %w", err)
		}
		want := c.Expected.Capabilities
		if !equalCaps(caps, want) {
			return fmt.Errorf("capabilities mismatch: got %v, want %v", caps, want)
		}
		return nil
	}

	if s.Kind != "negative" {
		return fmt.Errorf("kind %q: rejection expected only by negative fixtures", s.Kind)
	}
	slot := &declaredSlot{caps: toCoreCaps(c.Declaration)}
	if _, err := reg.Admit(c.Name, slot); err == nil {
		return fmt.Errorf("invalid declaration was admitted: %v", c.Declaration)
	}
	return nil
}

func equalCaps(got []registry.Capability, want []capability) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i].Name != want[i].Name || got[i].Version != want[i].Version {
			return false
		}
	}
	return true
}

func toCoreCaps(caps []capability) []*corev1.Capability {
	out := make([]*corev1.Capability, 0, len(caps))
	for _, c := range caps {
		out = append(out, &corev1.Capability{Name: c.Name, Version: c.Version})
	}
	return out
}

type declaredSlot struct {
	corev1.UnimplementedMetaServiceServer
	caps []*corev1.Capability
}

func (d *declaredSlot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{Capabilities: d.caps}, nil
}
