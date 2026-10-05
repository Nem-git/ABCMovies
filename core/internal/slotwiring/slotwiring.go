// Package slotwiring is the composition glue between operator config and
// in-process adapter implementations (TECHNICAL-DECISIONS.md §1.3). Each
// adapter ships one wiring file that registers a factory under its adapter
// name; SetupProviders walks the configured provider entries and hands each
// to its factory. Adding another instance of an existing adapter is pure
// configuration; adding a new adapter is its own package plus one Register
// call — no existing file changes.
//
// This package is deliberately the ONLY place that knows both sides: it may
// import core internals and adapters, because adapters themselves stay pure
// (HTTP + generated proto only) and the core never imports adapters.
package slotwiring

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/accounts"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/enrichment"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// Deps is everything an adapter's factory may need from the composition.
type Deps struct {
	Ctx      context.Context
	Registry *registry.InProcessRegistry
	// Accounts is the instance's linked-account store (PLAN.md §3.5). It
	// doubles as the session vault provider slots persist validated sessions
	// through, so a linked account never needs a password env at boot.
	Accounts *accounts.Store
	// LinkedBySlot holds the linked-account routing result computed before
	// the provider factories run: enabled slot id -> the linked accounts that
	// attach to it. Factories for adapters without linked accounts see an
	// empty (or absent) slice.
	LinkedBySlot map[string][]accounts.Record
	SourceCache  store.Store
	Logger       *slog.Logger
	// ItemRegistry is the instance-wide provider item registry (identity is
	// global state, not per-slot). Provider factories require it.
	ItemRegistry *itemregistry.Registry
	// EventSink receives availability events emitted by source-cache syncs;
	// nil drops them.
	EventSink sourcecache.EventSink
	// Enqueue hands entry IDs to the enrichment queue (T2 trigger,
	// TECHNICAL-DECISIONS.md §1.28): after identity work produced or
	// changed a mapping, its entry becomes an enrichment candidate. Nil
	// disables the trigger (no catalogue slots configured).
	Enqueue func(entryID string)
}

// builtSlot is a provider slot fully assembled by its factory but not yet
// published: the adapter instance, its per-account sync machinery (one
// source-cache synchronizer per account, each having run its initial sync),
// the handshake-declared cadence resolved into refresh jobs, and the
// delivery resolver. Nothing has been admitted into the registry, so a
// failure anywhere in the build publishes nothing.
type builtSlot struct {
	entry    config.SlotEntry
	impl     corev1.MetaServiceServer
	jobs     []scheduler.Job
	reaches  []library.Reach
	resolver delivery.Resolver
}

// providerFactory builds one slot instance and everything derived from it,
// but does NOT admit it: admission is the caller's last step, so a factory
// that fails halfway leaves the instance unwired rather than half-wired.
type providerFactory func(entry config.SlotEntry, deps Deps) (*builtSlot, error)

var providers = map[string]providerFactory{}

// RegisterProvider wires an adapter implementation to its config name. Called
// from each adapter's wiring file via init().
func RegisterProvider(adapter string, f providerFactory) {
	if _, dup := providers[adapter]; dup {
		panic(fmt.Sprintf("slotwiring: provider adapter %q registered twice", adapter))
	}
	providers[adapter] = f
}

// builtCatalogue is the engine-facing catalogue client paired with the
// served slot instance, ready to be admitted.
type builtCatalogue struct {
	catalogue enrichment.Catalogue
	impl      corev1.MetaServiceServer
}

// catalogueFactory builds one catalogue slot instance (its token check and
// construction) and hands back the engine-facing client pair without
// admitting it. Catalogues run no jobs of their own — they are pulled by the
// enrichment drain, not pushed by a cadence.
type catalogueFactory func(entry config.SlotEntry, deps Deps) (*builtCatalogue, error)

var catalogs = map[string]catalogueFactory{}

// RegisterCatalogue wires a catalogue adapter implementation to its config
// name. Called from each adapter's wiring file via init().
func RegisterCatalogue(adapter string, f catalogueFactory) {
	if _, dup := catalogs[adapter]; dup {
		panic(fmt.Sprintf("slotwiring: catalogue adapter %q registered twice", adapter))
	}
	catalogs[adapter] = f
}

// namespaceClaimer is implemented by catalogue adapters that can resolve
// foreign identity namespaces; it powers the no-overlap rule below.
type namespaceClaimer interface{ Namespaces() []string }

// SetupCatalogues builds, validates and admits every enabled catalogue entry.
// Two enabled slots may never claim the same identity namespace — with
// overlap, GetMetadata(ref) would silently depend on wiring order instead of
// data (TECHNICAL-DECISIONS.md §1.29), so startup fails loudly instead. The
// overlap check runs on the served slot instance (not the narrowed engine
// client), before anything is admitted.
func SetupCatalogues(entries []config.SlotEntry, deps Deps) ([]enrichment.Catalogue, error) {
	logger := deps.Logger

	type pending struct {
		entry config.SlotEntry
		b     *builtCatalogue
	}
	var built []pending
	for _, entry := range entries {
		if !entry.Enabled {
			logger.Info("slot disabled by config; skipping", "slot", entry.ID, "adapter", entry.Adapter)
			continue
		}
		f, ok := catalogs[entry.Adapter]
		if !ok {
			return nil, fmt.Errorf("slot %q: unknown catalogue adapter %q (registered: %v)", entry.ID, entry.Adapter, keys(catalogs))
		}
		cat, err := f(entry, deps)
		if err != nil {
			return nil, fmt.Errorf("slot %q (adapter %q): %w", entry.ID, entry.Adapter, err)
		}
		built = append(built, pending{entry, cat})
	}

	claimed := map[string]string{} // namespace -> slot id
	for _, p := range built {
		if claimer, ok := p.b.impl.(namespaceClaimer); ok {
			for _, ns := range claimer.Namespaces() {
				if owner, dup := claimed[ns]; dup {
					return nil, fmt.Errorf("slots %q and %q both claim identity namespace %q", owner, p.entry.ID, ns)
				}
				claimed[ns] = p.entry.ID
			}
		}
	}

	var out []enrichment.Catalogue
	for _, p := range built {
		caps, err := deps.Registry.Admit(p.entry.ID, p.b.impl)
		if err != nil {
			return nil, fmt.Errorf("slot %q (adapter %q): %w", p.entry.ID, p.entry.Adapter, err)
		}
		logAdmitted(logger, p.entry.ID, caps)
		out = append(out, p.b.catalogue)
	}
	return out, nil
}

// RouteLinkedAccounts assigns each linked provider account to exactly one
// enabled slot instance of the matching adapter whose declared server it
// belongs to — or, when no configured slot serves that server, hands the
// record back as a provisioning seed (the accounts of that server become
// their own user-owned server slot; PLAN.md §3.5 sharing decision). The rule
// is deterministic:
//
//   - no enabled slot of that adapter declares the record's server ->
//     provisioned: the caller wires the record as a user-owned server under
//     ServerNamespace;
//   - exactly one enabled slot declares it -> attached there;
//   - several enabled slots declare it -> a wiring error, never a silent pick.
//
// Routing is per server: the slot id is the identity namespace (§1.25), so an
// item seen through a linked account must join the same namespace as the
// operator-declared accounts of the same server — otherwise the same film
// from two accounts of one server would split into two identities.
func RouteLinkedAccounts(entries []config.SlotEntry, records []accounts.Record) (bySlot map[string][]accounts.Record, provisioned []accounts.Record, err error) {
	bySlot = map[string][]accounts.Record{}
	for _, rec := range records {
		var matching []string
		for _, e := range entries {
			if e.Enabled && e.Adapter == rec.Provider && canonicalServer(e.Server) == canonicalServer(rec.BaseURL) {
				matching = append(matching, e.ID)
			}
		}
		switch len(matching) {
		case 1:
			bySlot[matching[0]] = append(bySlot[matching[0]], rec)
		case 0:
			provisioned = append(provisioned, rec)
		default:
			return nil, nil, fmt.Errorf(
				"linked %s account %q (base-url %q) is ambiguous: slots %v all declare that server",
				rec.Provider, rec.ID, rec.BaseURL, matching)
		}
	}
	return bySlot, provisioned, nil
}

// ServerNamespace derives the deterministic identity namespace for a
// user-owned server (PLAN.md §1.25): the canonical server identity, never the
// account or its owner. Every account of one server — and every user who
// links it — lands in the same namespace, so the same film seen through any
// of them merges into one entry. It is stable across reboots and doubles as
// the slot id a provisioned user-owned server is wired under. The name is
// readable (srv-<adapter>-<host>-<hash>) because it appears in logs and
// error messages; the hash tail keeps two servers on one host distinct.
func ServerNamespace(rec accounts.Record) string {
	base := canonicalServer(rec.BaseURL)
	h := sha256.Sum256([]byte(rec.Provider + "\x00" + base))
	return fmt.Sprintf("srv-%s-%s-%s", rec.Provider, serverSlug(base), hex.EncodeToString(h[:4]))
}

// serverSlug makes a server authority int8 legible: lowercase, non-alphanumer
// characters become dashes, nothing leading or trailing. Used inside derived
// slot names only; the hash keeps identity canonical.
func serverSlug(base string) string {
	u, err := url.Parse(base)
	authority := base
	if err == nil && u.Host != "" {
		authority = u.Host
	}
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(authority) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case !dash && b.Len() > 0:
			b.WriteByte('-')
			dash = true
		}
	}
	if out := strings.Trim(b.String(), "-"); out != "" {
		return out
	}
	return "unknown"
}

// canonicalServer normalizes a base URL enough to be a stable namespace
// identity: scheme, lowercased host, and path with its trailing slash trimmed.
// Unparsable input degrades to the trimmed, lowercased string itself.
func canonicalServer(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return strings.ToLower(strings.TrimRight(base, "/"))
	}
	return u.Scheme + "://" + strings.ToLower(u.Host) + strings.TrimRight(u.Path, "/")
}

// Resolvers maps a provider slot id to its produce-sources delivery resolver,
// so the delivery engine can route a provider/account/native_id to the right
// adapter (identity is the slot instance id, TECHNICAL-DECISIONS.md §1.25).
type Resolvers map[string]delivery.Resolver

// SetupProviders admits every enabled provider entry and returns the jobs
// implementing their refresh cadence plus the reaches their accounts expose.
// An unknown adapter or a failing handshake aborts startup loudly — a
// half-wired instance is worse than a down one.
func SetupProviders(entries []config.SlotEntry, deps Deps) ([]scheduler.Job, []library.Reach, Resolvers, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	deps.Logger = logger
	if deps.Ctx == nil {
		deps.Ctx = context.Background()
	}

	// Route the linked accounts to their slots before any factory runs: the
	// assignment is a global decision (several slots of one adapter), while a
	// factory only ever sees its own entry — so the result travels on Deps.
	// A link whose server no configured slot serves is the request to provision
	// a user-owned server: it is wired as its own synthetic slot keyed by the
	// server's derived namespace (PLAN.md §3.5).
	if deps.Accounts != nil {
		linked, err := deps.Accounts.List(deps.Ctx)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("linked accounts: %w", err)
		}
		bySlot, provisioned, err := RouteLinkedAccounts(entries, linked)
		if err != nil {
			return nil, nil, nil, err
		}
		deps.LinkedBySlot = bySlot
		// Provisioned records group by derived server id: two accounts on one
		// server — the ordinary household case where several members link the
		// same home server — become ONE synthetic slot carrying them all,
		// never two slots claiming the same namespace (which would refuse the
		// second at Admit and kill boot).
		grouped := map[string][]accounts.Record{}
		var serverOrder []string
		for _, rec := range provisioned {
			ns := ServerNamespace(rec)
			if _, ok := grouped[ns]; !ok {
				serverOrder = append(serverOrder, ns)
			}
			grouped[ns] = append(grouped[ns], rec)
		}
		for _, ns := range serverOrder {
			recs := grouped[ns]
			if _, ok := providers[recs[0].Provider]; !ok {
				logger.Warn("linked account's provider adapter is not registered; it stays stored but feeds no library",
					"server", ns, "provider", recs[0].Provider, "accounts", len(recs))
				continue
			}
			deps.LinkedBySlot[ns] = append(deps.LinkedBySlot[ns], recs...)
			// The synthetic entry carries no operator accounts: the linked
			// records ARE the slot's vault-first accounts.
			entries = append(entries, config.SlotEntry{
				Adapter:   recs[0].Provider,
				ID:        ns,
				Enabled:   true,
				Transport: "in-process",
				Server:    recs[0].BaseURL,
			})
			logger.Info("linked accounts provision one user-owned server slot",
				"server", ns, "base_url", recs[0].BaseURL, "accounts", len(recs))
		}
	}

	var built []*builtSlot
	for _, entry := range entries {
		if !entry.Enabled {
			logger.Info("slot disabled by config; skipping", "slot", entry.ID, "adapter", entry.Adapter)
			continue
		}
		f, ok := providers[entry.Adapter]
		if !ok {
			return nil, nil, nil, fmt.Errorf("slot %q: unknown provider adapter %q (registered: %v)", entry.ID, entry.Adapter, keys(providers))
		}
		b, err := f(entry, deps)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("slot %q (adapter %q): %w", entry.ID, entry.Adapter, err)
		}
		built = append(built, b)
	}

	// Publish last: every factory succeeded before any slot is admitted, so a
	// failure above leaves the registry untouched rather than half-populated.
	var jobs []scheduler.Job
	var reaches []library.Reach
	resolvers := Resolvers{}
	for _, b := range built {
		caps, err := deps.Registry.Admit(b.entry.ID, b.impl)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("slot %q (adapter %q): %w", b.entry.ID, b.entry.Adapter, err)
		}
		logAdmitted(logger, b.entry.ID, caps)
		jobs = append(jobs, b.jobs...)
		reaches = append(reaches, b.reaches...)
		if b.resolver != nil {
			resolvers[b.entry.ID] = b.resolver
		}
	}
	return jobs, reaches, resolvers, nil
}

// SetupAll walks every slot kind from config. Provider and catalogue wiring
// are implemented, as are sinks (their factory resolves the configured disk
// and device entries); the remaining kinds are stubs that fail loudly if an
// operator ever declares one before its milestone lands — silent ignoring
// would make a typo look like a working deployment.
func SetupAll(ctx context.Context, slots config.SlotsConfig, deps Deps) ([]scheduler.Job, []library.Reach, []enrichment.Catalogue, Resolvers, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	deps.Ctx = ctx
	deps.Logger = logger

	pJobs, reaches, resolvers, err := SetupProviders(slots.Providers, deps)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	cats, err := SetupCatalogues(slots.Catalogue, deps)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	// Sinks are wired through SetupSinks (they need a delivery relay), so
	// SetupAll deliberately does not build the sink factory here.

	for _, kind := range []struct {
		name    string
		entries []config.SlotEntry
	}{
		{"subtitle-source", slots.SubtitleSources},
		{"drm", slots.Drm},
	} {
		if len(kind.entries) > 0 {
			return nil, nil, nil, nil, fmt.Errorf("%s slots are not implemented yet; remove the %q entries or wait for their milestone", kind.name, kind.name+"s")
		}
	}
	return pJobs, reaches, cats, resolvers, nil
}

// DeclaredCadence resolves a sync cadence by precedence: explicit operator
// config wins over the adapter's handshake-declared policy, which wins over
// the scheduler default (zero). A declared value that fails to parse is a
// broken adapter or config and is reported, not ignored.
func DeclaredCadence(configValue string, declared map[string]string, key string) (time.Duration, error) {
	if configValue != "" {
		d, err := time.ParseDuration(configValue)
		if err != nil || d <= 0 {
			return 0, fmt.Errorf("sync-cadence %q is not a positive duration", configValue)
		}
		return d, nil
	}
	if v := declared[key]; v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return 0, fmt.Errorf("adapter declared invalid %s %q", key, v)
		}
		return d, nil
	}
	return 0, nil // scheduler default
}

func keys[M ~map[string]V, V any](m M) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// logAdmitted prints a slot's handshake result at boot so an operator can see
// exactly what the running instance declared.
func logAdmitted(logger *slog.Logger, slot string, caps []registry.Capability) {
	names := make([]string, 0, len(caps))
	for _, c := range caps {
		names = append(names, fmt.Sprintf("%s v%d", c.Name, c.Version))
	}
	logger.Info("slot admitted", "slot", slot, "capabilities", names)
}
