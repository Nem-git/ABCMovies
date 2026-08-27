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
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/nem-git/abcmovies/core/internal/config"
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
	Ctx         context.Context
	Registry    *registry.InProcessRegistry
	SealedBlobs interface {
		Save(ctx context.Context, key string, blob []byte) error
		Load(ctx context.Context, key string) ([]byte, error)
	}
	SourceCache store.Store
	Logger      *slog.Logger
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
// derived libraries.
type providerFactory func(entry config.SlotEntry, deps Deps) ([]scheduler.Job, []library.Reach, error)

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

// SetupProviders admits every enabled provider entry and returns the jobs
// implementing their refresh cadence plus the reaches their accounts expose.
// An unknown adapter or a failing handshake aborts startup loudly — a
// half-wired instance is worse than a down one.
func SetupProviders(entries []config.SlotEntry, deps Deps) ([]scheduler.Job, []library.Reach, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	deps.Logger = logger
	if deps.Ctx == nil {
		deps.Ctx = context.Background()
	}

	var jobs []scheduler.Job
	var reaches []library.Reach
	for _, entry := range entries {
		if !entry.Enabled {
			logger.Info("slot disabled by config; skipping", "slot", entry.ID, "adapter", entry.Adapter)
			continue
		}
		f, ok := providers[entry.Adapter]
		if !ok {
			return nil, nil, fmt.Errorf("slot %q: unknown provider adapter %q (registered: %v)", entry.ID, entry.Adapter, keys(providers))
		}
		entryJobs, entryReaches, err := f(entry, deps)
		if err != nil {
			return nil, nil, fmt.Errorf("slot %q (adapter %q): %w", entry.ID, entry.Adapter, err)
		}
		jobs = append(jobs, entryJobs...)
		reaches = append(reaches, entryReaches...)
	}
	return jobs, reaches, nil
}

// SetupAll walks every slot kind from config. Provider and catalogue wiring
// are implemented; the remaining kinds are stubs that fail loudly if an
// operator ever declares one before its milestone lands — silent ignoring
// would make a typo look like a working deployment.
func SetupAll(ctx context.Context, slots config.SlotsConfig, deps Deps) ([]scheduler.Job, []library.Reach, []enrichment.Catalogue, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	deps.Ctx = ctx
	deps.Logger = logger

	pJobs, reaches, err := SetupProviders(slots.Providers, deps)
	if err != nil {
		return nil, nil, nil, err
	}

	cats, err := SetupCatalogues(slots.Catalogue, deps)
	if err != nil {
		return nil, nil, nil, err
	}

	for _, kind := range []struct {
		name    string
		entries []config.SlotEntry
	}{
		{"sink", slots.Sinks},
		{"subtitle-source", slots.SubtitleSources},
		{"drm", slots.Drm},
	} {
		if len(kind.entries) > 0 {
			return nil, nil, nil, fmt.Errorf("%s slots are not implemented yet; remove the %q entries or wait for their milestone", kind.name, kind.name+"s")
		}
	}
	return pJobs, reaches, cats, nil
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
