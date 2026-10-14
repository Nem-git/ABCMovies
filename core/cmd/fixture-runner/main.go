package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/nem-git/abcmovies/adapters/jellyfin"
	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/builtin"
	"github.com/nem-git/abcmovies/core/internal/fakejellyfin"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/schema"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
	"google.golang.org/protobuf/encoding/protojson"
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
	// Accounts and ExpectedLibrary drive pipeline suites (library_merge):
	// each account's items are synced through the real source cache → item
	// registry pipeline, then the derived per-user library is compared
	// against the expectation.
	Accounts        []mergeAccount `json:"accounts"`
	ExpectedLibrary *mergeLibrary  `json:"expected_library"`
	// Behavioural conformance fields. When RequestData is present the runner
	// exercises the real adapter against a fake backend instead of only
	// validating the message shape.
	RequestData json.RawMessage `json:"request_data"`
	// CatalogueItems describe the fake Jellyfin server's catalogue for
	// behavioural provider fixtures.
	CatalogueItems []catalogueItem `json:"catalogue_items"`
}

// catalogueItem is the subset of Jellyfin's BaseItemDto the fake index holds.
type catalogueItem struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	Name      string            `json:"name"`
	Year      int               `json:"year"`
	Providers map[string]string `json:"providers"`
}

// mergeAccount is one linked account of one provider slot in a pipeline
// fixture. The provider string is the slot-scoped namespace
// (TECHNICAL-DECISIONS.md §1.25).
type mergeAccount struct {
	ID       string            `json:"id"`
	Provider string            `json:"provider"`
	Items    []json.RawMessage `json:"items"`
}

type mergeLibrary struct {
	Entries []entrySummary `json:"entries"`
}

type entrySummary struct {
	Kind               string                `json:"kind"`
	ExternalIdentities []identitySummary     `json:"external_identities"`
	Coverage           map[string]rowSummary `json:"coverage"`
}

type identitySummary struct {
	Namespace string `json:"namespace"`
	Value     string `json:"value"`
	Verdict   string `json:"verdict"`
}

type rowSummary struct {
	Present bool     `json:"present"`
	Verdict string   `json:"verdict"`
	Via     []string `json:"via"`
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
	case "library_merge":
		return runLibraryMergeCase(s, c)
	case "provider":
		return runProviderCase(s, c)
	case "catalogue":
		return runCatalogueCase(s, c)
	case "drm":
		return runDRMCase(s, c)
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
// runLibraryMergeCase executes pipeline fixtures: every account's catalogue
// is synced through the production source cache and item registry, the
// per-user library is derived exactly as main.go derives it, and the result
// is compared entry-for-entry against the expectation. Positive cases pin
// merges that must happen; negative cases pin separations that must not be
// merged (no merge without corroboration — THREAT-MODEL.md T14).
func runLibraryMergeCase(s suite, c fixture) error {
	if c.ExpectedLibrary == nil {
		return fmt.Errorf("expected_library is required for pipeline suites")
	}
	ctx := context.Background()
	st := store.NewInMemory()
	reg, err := itemregistry.New(st, "")
	if err != nil {
		return fmt.Errorf("registry: %w", err)
	}

	reaches := make([]library.Reach, 0, len(c.Accounts))
	for _, acct := range c.Accounts {
		items := make([]*slotsv1.CatalogueItem, 0, len(acct.Items))
		for i, raw := range acct.Items {
			var item slotsv1.CatalogueItem
			if err := protojson.Unmarshal(raw, &item); err != nil {
				return fmt.Errorf("account %q item %d: %w", acct.ID, i, err)
			}
			items = append(items, &item)
		}
		syncer, err := sourcecache.New(acct.Provider, &cannedProvider{items}, st, slog.Default(),
			sourcecache.WithEntryLookup(reg),
			sourcecache.WithItemResolver(registryResolver{reg}))
		if err != nil {
			return fmt.Errorf("source cache: %w", err)
		}
		if _, err := syncer.SyncAccount(ctx, acct.ID); err != nil {
			return fmt.Errorf("sync %q: %w", acct.ID, err)
		}
		// Fixture accounts stand in for host-provided (operator-declared)
		// slots: public by §2.2, deriving into every user's library.
		reaches = append(reaches, library.Reach{Sync: syncer, AccountID: acct.ID, Provider: "jellyfin", Visibility: accounts.VisibilityPublic})
	}

	svc, err := library.NewService(reaches, reg, store.NewInMemory(), slog.Default())
	if err != nil {
		return fmt.Errorf("library service: %w", err)
	}
	lib, err := svc.Library(ctx, "fixture-user")
	if err != nil {
		return fmt.Errorf("derive library: %w", err)
	}

	got := summarize(lib)
	want := c.ExpectedLibrary.Entries
	sortEntries(got)
	sortEntries(want)
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("derived library mismatch:\n got: %s\nwant: %s", render(got), render(want))
	}
	return nil
}

type cannedProvider struct{ items []*slotsv1.CatalogueItem }

func (p *cannedProvider) CatalogueSync(_ context.Context, _ *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	return &slotsv1.CatalogueSyncResponse{Items: p.items}, nil
}

// registryResolver adapts the item registry to the synchronizer's
// ItemResolver.
type registryResolver struct{ r *itemregistry.Registry }

func (p registryResolver) Resolve(ctx context.Context, provider string, item *slotsv1.CatalogueItem) error {
	_, err := p.r.Resolve(ctx, provider, item)
	return err
}

func summarize(entries []*corev1.LibraryEntry) []entrySummary {
	out := make([]entrySummary, 0, len(entries))
	for _, e := range entries {
		sum := entrySummary{
			Kind:               e.GetKind().String(),
			ExternalIdentities: []identitySummary{},
			Coverage:           map[string]rowSummary{},
		}
		for _, id := range e.GetExternalIdentities() {
			sum.ExternalIdentities = append(sum.ExternalIdentities, identitySummary{
				Namespace: id.GetNamespace(),
				Value:     id.GetValue(),
				Verdict:   id.GetVerdict().String(),
			})
		}
		sort.Slice(sum.ExternalIdentities, func(i, j int) bool {
			a, b := sum.ExternalIdentities[i], sum.ExternalIdentities[j]
			if a.Namespace != b.Namespace {
				return a.Namespace < b.Namespace
			}
			return a.Value < b.Value
		})
		for key, row := range e.GetCoverage() {
			via := append([]string(nil), row.GetVia()...)
			sort.Strings(via)
			sum.Coverage[key] = rowSummary{Present: row.GetPresent(), Verdict: row.GetVerdict().String(), Via: via}
		}
		out = append(out, sum)
	}
	return out
}

func sortEntries(entries []entrySummary) {
	sort.Slice(entries, func(i, j int) bool { return render([]entrySummary{entries[i]}) < render([]entrySummary{entries[j]}) })
}

func render(entries []entrySummary) string {
	b, _ := json.Marshal(entries)
	return string(b)
}

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
		// Behavioural conformance: when the fixture carries request_data and
		// catalogue_items, exercise the real Jellyfin adapter against a fake
		// backend and compare the actual response against the expected message.
		if len(c.RequestData) > 0 && len(c.CatalogueItems) > 0 {
			return runProviderBehaviouralCase(c)
		}
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

// runProviderBehaviouralCase exercises the real Jellyfin adapter against a
// fake Jellyfin server seeded with the fixture's catalogue items. It sends
// the fixture's request through the adapter's CatalogueSync and compares the
// actual response against the fixture's expected message. This proves the
// adapter implements the contract behaviourally, not just that the message
// shape is schema-valid.
func runProviderBehaviouralCase(c fixture) error {
	ctx := context.Background()

	// Boot the fake Jellyfin server with the fixture's catalogue.
	items := make([]fakejellyfin.CatalogItem, len(c.CatalogueItems))
	for i, it := range c.CatalogueItems {
		items[i] = fakejellyfin.CatalogItem{
			ID:        it.ID,
			Type:      it.Type,
			Name:      it.Name,
			Year:      it.Year,
			Providers: it.Providers,
		}
	}
	fake := fakejellyfin.New(items)
	defer fake.Close()

	// Parse the request.
	var req slotsv1.CatalogueSyncRequest
	if err := protojson.Unmarshal(c.RequestData, &req); err != nil {
		return fmt.Errorf("parse request: %w", err)
	}

	// Build the Jellyfin adapter pointing at the fake server.
	accountID := req.GetAccountId()
	if accountID == "" {
		return fmt.Errorf("request missing account_id")
	}
	// Set a PasswordEnv name and provide the password through the
	// environment so the adapter performs a fresh login against the fake
	// server instead of expecting a pre-existing vaulted session.
	passwordEnv := "FIXTURE_RUNNER_JELLYFIN_PASSWORD"
	if err := os.Setenv(passwordEnv, "test-password"); err != nil {
		return fmt.Errorf("set %s: %w", passwordEnv, err)
	}
	src := singleAccountSource{
		Account: jellyfin.Account{
			ID:          accountID,
			URL:         fake.URL(),
			Username:    "testuser",
			PasswordEnv: passwordEnv,
		},
	}
	slot, err := jellyfin.New([]string{accountID}, src)
	if err != nil {
		return fmt.Errorf("jellyfin.New: %w", err)
	}

	// Drive the adapter's real code path.
	resp, err := slot.CatalogueSync(ctx, &req)
	if err != nil {
		return fmt.Errorf("CatalogueSync: %w", err)
	}

	// Marshal the actual response and compare against the expected message.
	// UseProtoNames ensures snake_case field names, matching the fixture format.
	// EmitUnpopulated includes empty fields (e.g. next_page_token: "").
	actual, err := protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal actual response: %w", err)
	}

	// Compare semantically: strip empty fields from both sides, compact,
	// and compare. This makes the comparison robust to EmitUnpopulated
	// (which includes empty strings, zero values, empty maps/slices) while
	// still proving every meaningful field matches exactly.
	expectedStripped := stripEmptyFields(c.Message)
	actualStripped := stripEmptyFields(actual)
	var expectedBuf, actualBuf bytes.Buffer
	if err := json.Compact(&expectedBuf, expectedStripped); err != nil {
		return fmt.Errorf("compact expected message: %w", err)
	}
	if err := json.Compact(&actualBuf, actualStripped); err != nil {
		return fmt.Errorf("compact actual message: %w", err)
	}
	if !bytes.Equal(actualBuf.Bytes(), expectedBuf.Bytes()) {
		return fmt.Errorf("behavioural mismatch:\n got: %s\nwant: %s",
			actualBuf.String(), expectedBuf.String())
	}
	return nil
}

// stripEmptyFields removes empty-valued fields (empty strings, zero numbers,
// empty slices, empty maps) from a JSON document. This lets behavioural
// comparisons ignore fields that EmitUnpopulated would otherwise include.
func stripEmptyFields(raw json.RawMessage) json.RawMessage {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	return json.RawMessage(compactJSON(stripEmpty(v)))
}

// stripEmpty recursively removes empty-valued fields from an unmarshaled JSON
// value.
func stripEmpty(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(val))
		for k, child := range val {
			sc := stripEmpty(child)
			if isEmptyValue(sc) {
				continue
			}
			out[k] = sc
		}
		return out
	case []interface{}:
		out := make([]interface{}, 0, len(val))
		for _, item := range val {
			out = append(out, stripEmpty(item))
		}
		return out
	default:
		return val
	}
}

// isEmptyValue reports whether a stripped JSON value is "empty" — an empty
// string, zero number, empty slice, or empty map.
func isEmptyValue(v interface{}) bool {
	switch val := v.(type) {
	case string:
		return val == ""
	case float64:
		return val == 0
	case bool:
		return !val
	case nil:
		return true
	case []interface{}:
		return len(val) == 0
	case map[string]interface{}:
		return len(val) == 0
	default:
		return false
	}
}

// compactJSON marshals a value back to compact JSON bytes.
func compactJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// singleAccountSource resolves every account id to the same fixed account.
type singleAccountSource struct {
	Account jellyfin.Account
}

func (s singleAccountSource) Lookup(_ context.Context, _ string) (jellyfin.Account, error) {
	return s.Account, nil
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
	case "ProduceSourcesRequest":
		var m slotsv1.ProduceSourcesRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateProduceSourcesRequest(&m)
		return err == nil, err
	case "ProduceSourcesResponse":
		var m slotsv1.ProduceSourcesResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateProduceSourcesResponse(&m)
		return err == nil, err
	case "SearchCatalogRequest":
		var m slotsv1.SearchCatalogRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateSearchCatalogRequest(&m)
		return err == nil, err
	case "SearchCatalogResponse":
		var m slotsv1.SearchCatalogResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateSearchCatalogResponse(&m)
		return err == nil, err
	case "BrowseCatalogRequest":
		var m slotsv1.BrowseCatalogRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateBrowseCatalogRequest(&m)
		return err == nil, err
	case "BrowseCatalogResponse":
		var m slotsv1.BrowseCatalogResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateBrowseCatalogResponse(&m)
		return err == nil, err
	case "RefreshAvailabilityRequest":
		var m slotsv1.RefreshAvailabilityRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateRefreshAvailabilityRequest(&m)
		return err == nil, err
	case "RefreshAvailabilityResponse":
		var m slotsv1.RefreshAvailabilityResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateRefreshAvailabilityResponse(&m)
		return err == nil, err
	default:
		return false, fmt.Errorf("unknown provider message type %q", msgType)
	}
}

func runDRMCase(s suite, c fixture) error {
	switch s.Kind {
	case "positive", "negative":
		if c.Expected.Valid == nil {
			return fmt.Errorf("expected.valid is required for drm schema suites")
		}
		valid, err := validateDRMMessage(c.Type, c.Message)
		if valid != *c.Expected.Valid {
			if *c.Expected.Valid {
				return fmt.Errorf("expected valid:true, got invalid: %v", err)
			}
			return fmt.Errorf("expected valid:false, but the message validated cleanly")
		}
		return nil
	default:
		return fmt.Errorf("kind %q is not supported for drm suites", s.Kind)
	}
}

func validateDRMMessage(msgType string, msg json.RawMessage) (bool, error) {
	switch msgType {
	case "AcquireKeysRequest":
		var m slotsv1.AcquireKeysRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateAcquireKeysRequest(&m)
		return err == nil, err
	case "AcquireKeysResponse":
		var m slotsv1.AcquireKeysResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateAcquireKeysResponse(&m)
		return err == nil, err
	case "DecryptRequest":
		var m slotsv1.DecryptRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateDecryptRequest(&m)
		return err == nil, err
	case "DecryptResponse":
		var m slotsv1.DecryptResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateDecryptResponse(&m)
		return err == nil, err
	default:
		return false, fmt.Errorf("unknown drm message type %q", msgType)
	}
}

func runCatalogueCase(s suite, c fixture) error {
	switch s.Kind {
	case "positive", "negative":
		if c.Expected.Valid == nil {
			return fmt.Errorf("expected.valid is required for catalogue schema suites")
		}
		valid, err := validateCatalogueMessage(c.Type, c.Message)
		if valid != *c.Expected.Valid {
			if *c.Expected.Valid {
				return fmt.Errorf("expected valid:true, got invalid: %v", err)
			}
			return fmt.Errorf("expected valid:false, but the message validated cleanly")
		}
		return nil
	default:
		return fmt.Errorf("kind %q is not supported for catalogue suites", s.Kind)
	}
}

func validateCatalogueMessage(msgType string, msg json.RawMessage) (bool, error) {
	switch msgType {
	case "LookupTitleRequest":
		var m slotsv1.LookupTitleRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateLookupTitleRequest(&m)
		return err == nil, err
	case "LookupTitleResponse":
		var m slotsv1.LookupTitleResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateLookupTitleResponse(&m)
		return err == nil, err
	case "GetMetadataRequest":
		var m slotsv1.GetMetadataRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetMetadataRequest(&m)
		return err == nil, err
	case "GetMetadataResponse":
		var m slotsv1.GetMetadataResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetMetadataResponse(&m)
		return err == nil, err
	default:
		return false, fmt.Errorf("unknown catalogue message type %q", msgType)
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
	case "StartDeliveryRequest":
		var m apiv1.StartDeliveryRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateStartDeliveryRequest(&m)
		return err == nil, err
	case "StartDeliveryResponse":
		var m apiv1.StartDeliveryResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateStartDeliveryResponse(&m)
		return err == nil, err
	case "HeartbeatRequest":
		var m apiv1.HeartbeatRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateHeartbeatRequest(&m)
		return err == nil, err
	case "HeartbeatResponse":
		var m apiv1.HeartbeatResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateHeartbeatResponse(&m)
		return err == nil, err
	case "GetLibraryRequest":
		var m apiv1.GetLibraryRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetLibraryRequest(&m)
		return err == nil, err
	case "GetLibraryResponse":
		var m apiv1.GetLibraryResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetLibraryResponse(&m)
		return err == nil, err
	case "LinkAccountRequest":
		var m apiv1.LinkAccountRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateLinkAccountRequest(&m)
		return err == nil, err
	case "LinkAccountResponse":
		var m apiv1.LinkAccountResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateLinkAccountResponse(&m)
		return err == nil, err
	case "ListAccountsRequest":
		var m apiv1.ListAccountsRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateListAccountsRequest(&m)
		return err == nil, err
	case "ListAccountsResponse":
		var m apiv1.ListAccountsResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateListAccountsResponse(&m)
		return err == nil, err
	case "RemoveAccountRequest":
		var m apiv1.RemoveAccountRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateRemoveAccountRequest(&m)
		return err == nil, err
	case "UpdateAccountRequest":
		var m apiv1.UpdateAccountRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateUpdateAccountRequest(&m)
		return err == nil, err
	case "UpdateAccountResponse":
		var m apiv1.UpdateAccountResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateUpdateAccountResponse(&m)
		return err == nil, err
	case "GetPlayInfoRequest":
		var m apiv1.GetPlayInfoRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetPlayInfoRequest(&m)
		return err == nil, err
	case "GetPlayInfoResponse":
		var m apiv1.GetPlayInfoResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetPlayInfoResponse(&m)
		return err == nil, err
	case "CreateSlotRequest":
		var m apiv1.CreateSlotRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateCreateSlotRequest(&m)
		return err == nil, err
	case "CreateSlotResponse":
		var m apiv1.CreateSlotResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateCreateSlotResponse(&m)
		return err == nil, err
	case "GetSlotRequest":
		var m apiv1.GetSlotRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetSlotRequest(&m)
		return err == nil, err
	case "GetSlotResponse":
		var m apiv1.GetSlotResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetSlotResponse(&m)
		return err == nil, err
	case "UpdateSlotRequest":
		var m apiv1.UpdateSlotRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateUpdateSlotRequest(&m)
		return err == nil, err
	case "UpdateSlotResponse":
		var m apiv1.UpdateSlotResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateUpdateSlotResponse(&m)
		return err == nil, err
	case "DeleteSlotRequest":
		var m apiv1.DeleteSlotRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateDeleteSlotRequest(&m)
		return err == nil, err
	case "DeleteSlotResponse":
		var m apiv1.DeleteSlotResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateDeleteSlotResponse(&m)
		return err == nil, err
	case "ListSlotsRequest":
		var m apiv1.ListSlotsRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateListSlotsRequest(&m)
		return err == nil, err
	case "ListSlotsResponse":
		var m apiv1.ListSlotsResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateListSlotsResponse(&m)
		return err == nil, err
	case "GetDeliveryOptionsRequest":
		var m apiv1.GetDeliveryOptionsRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateGetDeliveryOptionsRequest(&m)
		return err == nil, err
	case "GetDeliveryOptionsResponse":
		var m apiv1.GetDeliveryOptionsResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		return true, nil
	case "ListJobsRequest":
		var m apiv1.ListJobsRequest
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		err := schema.ValidateListJobsRequest(&m)
		return err == nil, err
	case "ListJobsResponse":
		var m apiv1.ListJobsResponse
		if err := protojson.Unmarshal(msg, &m); err != nil {
			return false, err
		}
		return true, nil
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
