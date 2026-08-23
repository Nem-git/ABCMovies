package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"google.golang.org/protobuf/encoding/protojson"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/schema"
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
	// Policy pins the operating policy a slot declared at handshake; absent
	// means "do not assert". Only keys listed here are compared.
	Policy map[string]string `json:"policy"`
	// Valid states whether a schema fixture must validate (PLAN.md §2.5).
	// Required for schema suites; ignored by handshake suites.
	Valid *bool `json:"valid"`
}

type fixture struct {
	Name        string            `json:"name"`
	Request     string            `json:"request"`
	Declaration []capability      `json:"declaration"`
	Policy      map[string]string `json:"policy"`
	Expected    expected          `json:"expected"`
	// Message is a protojson-encoded contract instance, used by schema suites.
	Message json.RawMessage `json:"message"`
	// Type identifies the message type for API contract validation.
	Type string `json:"type"`
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
	switch s.Contract {
	case "meta":
		return runMetaCase(s, c)
	case "library_entry", "media_source", "job", "event", "title_metadata":
		return runSchemaCase(s, c)
	case "provider":
		return runProviderCase(s, c)
	case "api":
		return runAPICase(s, c)
	default:
		return fmt.Errorf("unknown contract %q", s.Contract)
	}
}

// runSchemaCase validates one protojson-encoded contract instance against the
// schema rules (PLAN.md §2.5: reject, never downgrade). The suite kind must
// match the expected verdict, and a positive case that fails to parse is a
// broken fixture.
func runSchemaCase(s suite, c fixture) error {
	if c.Expected.Valid == nil {
		return fmt.Errorf("expected.valid is required for schema suites")
	}
	switch s.Kind {
	case "positive":
		if !*c.Expected.Valid {
			return fmt.Errorf("positive suite: case %q must expect valid:true", c.Name)
		}
	case "negative":
		if *c.Expected.Valid {
			return fmt.Errorf("negative suite: case %q must expect valid:false", c.Name)
		}
	default:
		return fmt.Errorf("kind %q is not supported for schema suites", s.Kind)
	}

	valid, err := validateContract(s.Contract, c.Message)
	if valid != *c.Expected.Valid {
		if *c.Expected.Valid {
			return fmt.Errorf("expected valid:true, got invalid: %v", err)
		}
		return fmt.Errorf("expected valid:false, but the message validated cleanly")
	}
	return nil
}

func validateContract(contract string, msg json.RawMessage) (bool, error) {
	valid := false
	var err error
	switch contract {
	case "library_entry":
		var m corev1.LibraryEntry
		err = protojson.Unmarshal(msg, &m)
		if err == nil {
			err = schema.ValidateLibraryEntry(&m)
		}
		valid = err == nil
	case "media_source":
		var m corev1.MediaSource
		err = protojson.Unmarshal(msg, &m)
		if err == nil {
			err = schema.ValidateMediaSource(&m)
		}
		valid = err == nil
	case "job":
		var m corev1.Job
		err = protojson.Unmarshal(msg, &m)
		if err == nil {
			err = schema.ValidateJob(&m)
		}
		valid = err == nil
	case "event":
		var m corev1.EventEnvelope
		err = protojson.Unmarshal(msg, &m)
		if err == nil {
			err = schema.ValidateEventEnvelope(&m)
		}
		valid = err == nil
	case "title_metadata":
		var m corev1.TitleMetadata
		err = protojson.Unmarshal(msg, &m)
		if err == nil {
			err = schema.ValidateTitleMetadata(&m)
		}
		valid = err == nil
	default:
		return false, fmt.Errorf("unknown contract %q", contract)
	}
	return valid, err
}

// runProviderCase executes provider-slot fixtures: handshake cases admit a
// slot declaring its capabilities and pin what the registry reports back;
// schema-style positive/negative cases validate request and response pages of
// the whole-catalogue sync contract (PLAN.md §5.4).
func runProviderCase(s suite, c fixture) error {
	switch s.Kind {
	case "handshake":
		reg := registry.NewInProcess()
		defer reg.Close()
		slot := &declaredSlot{caps: toCoreCaps(c.Declaration), policy: c.Policy}
		caps, err := reg.Admit(c.Name, slot)
		if !c.Expected.Admitted {
			if err == nil {
				return fmt.Errorf("invalid declaration was admitted: %v", c.Declaration)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("handshake: %w", err)
		}
		want := c.Expected.Capabilities
		got := make([]registry.Capability, 0, len(caps))
		for _, cap_ := range caps {
			got = append(got, registry.Capability{Name: cap_.Name, Version: cap_.Version})
		}
		if !equalCaps(got, want) {
			return fmt.Errorf("capabilities mismatch: got %v, want %v", got, want)
		}
		if len(c.Expected.Policy) > 0 {
			policy, _ := reg.Policy(c.Name)
			for k, want := range c.Expected.Policy {
				if policy[k] != want {
					return fmt.Errorf("policy %q mismatch: got %q, want %q", k, policy[k], want)
				}
			}
		}
		return nil
	case "positive", "negative":
		if c.Expected.Valid == nil {
			return fmt.Errorf("expected.valid is required for provider schema suites")
		}
		valid, err := validateProviderMessage(c.Type, c.Message)
		if valid != *c.Expected.Valid {
			if *c.Expected.Valid {
				return fmt.Errorf("expected valid:true, got invalid: %v", err)
			}
			return fmt.Errorf("expected valid:false, but the message validated cleanly")
		}
		return nil
	default:
		return fmt.Errorf("kind %q is not supported for provider suites", s.Kind)
	}
}

func validateProviderMessage(msgType string, msg json.RawMessage) (bool, error) {
	switch msgType {
	case "CatalogueSyncRequest":
		var m slotsv1.CatalogueSyncRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateCatalogueSyncRequest(&m)
		return err == nil, err
	case "CatalogueSyncResponse":
		var m slotsv1.CatalogueSyncResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateCatalogueSyncResponse(&m)
		return err == nil, err
	default:
		return false, fmt.Errorf("unknown provider message type %q", msgType)
	}
}

func runAPICase(s suite, c fixture) error {
	if c.Expected.Valid == nil {
		return fmt.Errorf("expected.valid is required for API suites")
	}
	switch s.Kind {
	case "positive":
		if !*c.Expected.Valid {
			return fmt.Errorf("positive suite: case %q must expect valid:true", c.Name)
		}
	case "negative":
		if *c.Expected.Valid {
			return fmt.Errorf("negative suite: case %q must expect valid:false", c.Name)
		}
	default:
		return fmt.Errorf("kind %q is not supported for API suites", s.Kind)
	}
	valid, err := validateAPIMessage(c.Type, c.Message)
	if valid != *c.Expected.Valid {
		if *c.Expected.Valid {
			return fmt.Errorf("expected valid:true, got invalid: %v", err)
		}
		return fmt.Errorf("expected valid:false, but the message validated cleanly")
	}
	return nil
}

func validateAPIMessage(msgType string, msg json.RawMessage) (bool, error) {
	switch msgType {
	case "GetJobRequest":
		var m apiv1.GetJobRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetJobRequest(&m)
		return err == nil, err
	case "GetJobResponse":
		var m apiv1.GetJobResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetJobResponse(&m)
		return err == nil, err
	case "SubscribeRequest":
		var m apiv1.SubscribeRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateSubscribeRequest(&m)
		return err == nil, err
	default:
		return false, fmt.Errorf("unknown API message type %q", msgType)
	}
}

func runMetaCase(s suite, c fixture) error {
	reg := registry.NewInProcess()
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
	caps   []*corev1.Capability
	policy map[string]string
}

func (d *declaredSlot) CapabilityQuery(_ context.Context, _ *corev1.CapabilityQueryRequest) (*corev1.CapabilityQueryResponse, error) {
	return &corev1.CapabilityQueryResponse{Capabilities: d.caps, Policy: d.policy}, nil
}
