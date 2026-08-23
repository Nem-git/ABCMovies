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
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
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
}

// providerFactory admits one slot instance and returns its recurring jobs.
type providerFactory func(entry config.SlotEntry, deps Deps) ([]scheduler.Job, error)

var providers = map[string]providerFactory{}

// RegisterProvider wires an adapter implementation to its config name. Called
// from each adapter's wiring file via init().
func RegisterProvider(adapter string, f providerFactory) {
	if _, dup := providers[adapter]; dup {
		panic(fmt.Sprintf("slotwiring: provider adapter %q registered twice", adapter))
	}
	providers[adapter] = f
}

// SetupProviders admits every enabled provider entry and returns the jobs
// implementing their refresh cadence. An unknown adapter or a failing
// handshake aborts startup loudly — a half-wired instance is worse than a
// down one.
func SetupProviders(entries []config.SlotEntry, deps Deps) ([]scheduler.Job, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	deps.Logger = logger
	if deps.Ctx == nil {
		deps.Ctx = context.Background()
	}

	var jobs []scheduler.Job
	for _, entry := range entries {
		if !entry.Enabled {
			logger.Info("slot disabled by config; skipping", "slot", entry.ID, "adapter", entry.Adapter)
			continue
		}
		f, ok := providers[entry.Adapter]
		if !ok {
			return nil, fmt.Errorf("slot %q: unknown provider adapter %q (registered: %v)", entry.ID, entry.Adapter, keys(providers))
		}
		entryJobs, err := f(entry, deps)
		if err != nil {
			return nil, fmt.Errorf("slot %q (adapter %q): %w", entry.ID, entry.Adapter, err)
		}
		jobs = append(jobs, entryJobs...)
	}
	return jobs, nil
}

// SetupAll walks every slot kind from config. Provider wiring is fully
// implemented; the remaining kinds are stubs that fail loudly if an operator
// ever declares one before its milestone lands — silent ignoring would make
// a typo look like a working deployment.
func SetupAll(ctx context.Context, slots config.SlotsConfig, deps Deps) ([]scheduler.Job, error) {
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	deps.Ctx = ctx
	deps.Logger = logger

	var jobs []scheduler.Job
	pJobs, err := SetupProviders(slots.Providers, deps)
	if err != nil {
		return nil, err
	}
	jobs = append(jobs, pJobs...)

	for _, kind := range []struct {
		name    string
		entries []config.SlotEntry
	}{
		{"catalogue", slots.Catalogue},
		{"sink", slots.Sinks},
		{"subtitle-source", slots.SubtitleSources},
		{"drm", slots.Drm},
	} {
		if len(kind.entries) > 0 {
			return nil, fmt.Errorf("%s slots are not implemented yet; remove the %q entries or wait for their milestone", kind.name, kind.name+"s")
		}
	}
	return jobs, nil
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
