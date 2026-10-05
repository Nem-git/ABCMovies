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

// providerFactory admits one slot instance and returns its recurring jobs
// plus the reaches (synchronizer + account pairs) it makes available for
// derived libraries, and — when the adapter can produce media sources — the
// delivery resolver the engine routes produce-sources through (PLAN.md §6.2).
type providerFactory func(entry config.SlotEntry, deps Deps) ([]scheduler.Job, []library.Reach, delivery.Resolver, error)

var providers = map[string]providerFactory{}

// RegisterProvider wires an adapter implementation to its config name. Called
// from each adapter's wiring file via init().
func RegisterProvider(adapter string, f providerFactory) {
	if _, dup := providers[adapter]; dup {
		panic(fmt.Sprintf("slotwiring: provider adapter %q registered twice", adapter))
	}
	providers[adapter] = f
}

// catalogueFactory admits one catalogue slot instance and hands back the
// engine-facing client pair. Catalogues run no jobs of their own — they are
// pulled by the enrichment drain, not pushed by a cadence.
type catalogueFactory func(entry config.SlotEntry, deps Deps) (enrichment.Catalogue, error)

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

// SetupCatalogues admits every enabled catalogue entry. Two enabled slots
// may never claim the same identity namespace — with overlap,
// GetMetadata(ref) would silently depend on wiring order instead of data
// (TECHNICAL-DECISIONS.md §1.29), so startup fails loudly instead.
func SetupCatalogues(entries []config.SlotEntry, deps Deps) ([]enrichment.Catalogue, error) {
	logger := deps.Logger
	claimed := map[string]string{} // namespace -> slot id
	var out []enrichment.Catalogue
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
		if claimer, ok := cat.Client.(namespaceClaimer); ok {
			for _, ns := range claimer.Namespaces() {
				if owner, dup := claimed[ns]; dup {
					return nil, fmt.Errorf("slots %q and %q both claim identity namespace %q", owner, entry.ID, ns)
				}
				claimed[ns] = entry.ID
			}
		}
		out = append(out, cat)
	}
	return out, nil
}

// RouteLinkedAccounts assigns each linked provider account to exactly one
// enabled slot instance of the matching adapter — or, when no configured slot
// serves its server, hands the record back as a provisioning seed (the
// account becomes its own user-owned server slot; PLAN.md §3.5 sharing
// decision). The rule is deterministic:
//
//   - no enabled slot of that adapter -> provisioned: the caller wires the
//     account as a user-owned server under ServerNamespace;
//   - exactly one enabled slot -> attached there;
//   - several enabled slots -> attached to the unique one that declares an
//     operator account with the same server base-url; no match or several
//     matches is a wiring error, never a silent pick.
//
// Routing is per server: the slot id is the identity namespace (§1.25), so an
// item seen through a linked account must join the same namespace as the
// operator-declared accounts of the same Jellyfin server — otherwise the same
// film from two accounts of one server would split into two identities.
func RouteLinkedAccounts(entries []config.SlotEntry, records []accounts.Record) (bySlot map[string][]accounts.Record, provisioned []accounts.Record, err error) {
	bySlot = map[string][]accounts.Record{}
	for _, rec := range records {
		var candidates []config.SlotEntry
		for _, e := range entries {
			if e.Enabled && e.Adapter == rec.Provider {
				candidates = append(candidates, e)
			}
		}
		switch len(candidates) {
		case 0:
			provisioned = append(provisioned, rec)
		case 1:
			bySlot[candidates[0].ID] = append(bySlot[candidates[0].ID], rec)
		default:
			var matching []string
			for _, e := range candidates {
				for _, a := range e.Accounts {
					if a.URL == rec.BaseURL {
						matching = append(matching, e.ID)
						break
					}
				}
			}
			switch len(matching) {
			case 1:
				bySlot[matching[0]] = append(bySlot[matching[0]], rec)
			case 0:
				return nil, nil, fmt.Errorf(
					"linked %s account %q (base-url %q) matches no enabled slot's server: %d %s slots are enabled, declare the server or disable the extras",
					rec.Provider, rec.ID, rec.BaseURL, len(candidates), rec.Provider)
			default:
				return nil, nil, fmt.Errorf(
					"linked %s account %q (base-url %q) is ambiguous: slots %v all declare that server",
					rec.Provider, rec.ID, rec.BaseURL, matching)
			}
		}
	}
	return bySlot, provisioned, nil
}

// ServerNamespace derives the deterministic identity namespace for a
// user-owned server (PLAN.md §1.25): the canonical server identity, never the
// account or its owner. Every account of one server — and every user who
// links it — lands in the same namespace, so the same film seen through any
// of them merges into one entry. It is stable across reboots and doubles as
// the slot id a provisioned user-owned server is wired under.
func ServerNamespace(rec accounts.Record) string {
	h := sha256.Sum256([]byte(rec.Provider + "\x00" + canonicalServer(rec.BaseURL)))
	return "srv_" + hex.EncodeToString(h[:8])
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
		for _, rec := range provisioned {
			if _, ok := providers[rec.Provider]; !ok {
				logger.Warn("linked account's provider adapter is not registered; it stays stored but feeds no library",
					"account", rec.ID, "provider", rec.Provider)
				continue
			}
			ns := ServerNamespace(rec)
			deps.LinkedBySlot[ns] = append(deps.LinkedBySlot[ns], rec)
			// The synthetic entry carries no operator accounts: the linked
			// record IS the slot's single vault-first account.
			entries = append(entries, config.SlotEntry{
				Adapter:   rec.Provider,
				ID:        ns,
				Enabled:   true,
				Transport: "in-process",
			})
			logger.Info("linked account provisions a user-owned server slot",
				"account", rec.ID, "owner", rec.OwnerUserID, "server", ns, "base_url", rec.BaseURL)
		}
	}

	var jobs []scheduler.Job
	var reaches []library.Reach
	resolvers := Resolvers{}
	for _, entry := range entries {
		if !entry.Enabled {
			logger.Info("slot disabled by config; skipping", "slot", entry.ID, "adapter", entry.Adapter)
			continue
		}
		f, ok := providers[entry.Adapter]
		if !ok {
			return nil, nil, nil, fmt.Errorf("slot %q: unknown provider adapter %q (registered: %v)", entry.ID, entry.Adapter, keys(providers))
		}
		entryJobs, entryReaches, res, err := f(entry, deps)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("slot %q (adapter %q): %w", entry.ID, entry.Adapter, err)
		}
		jobs = append(jobs, entryJobs...)
		reaches = append(reaches, entryReaches...)
		if res != nil {
			resolvers[entry.ID] = res
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
