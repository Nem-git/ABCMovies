// Slot composition: everything the running instance needs from its
// configured provider slots — identity state, the derived-library service,
// the event path between them, and the refresh jobs. The gRPC command calls
// ComposeSlots; embedders that want provider slots do the same. Stack.Build
// deliberately stays slot-free so a bare core remains composable.
package app

import (
	"context"
	"fmt"
	"log/slog"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/registry"
	"github.com/nem-git/abcmovies/core/internal/scheduler"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
	"github.com/nem-git/abcmovies/core/internal/store"
)

// eventPublisher is the slice of the bus the mux needs.
type eventPublisher interface {
	Publish(env *corev1.EventEnvelope)
}

// accountInvalidator is the slice of the library service the mux needs.
type accountInvalidator interface {
	InvalidateAccount(provider, accountID string) error
}

// eventMux routes sync-emitted availability events to their two consumers:
// the event bus, and derived-library invalidation for the affected account.
// The invalidator arrives after wiring completes (the library needs every
// reach first), so events seen before then only reach the bus — correct,
// because nothing has read a derived library yet.
type eventMux struct {
	bus eventPublisher
	lib accountInvalidator
	log *slog.Logger
}

func (m *eventMux) Publish(env *corev1.EventEnvelope) {
	if av := env.GetAvailability(); av != nil {
		m.bus.Publish(env)
		if m.lib != nil {
			if err := m.lib.InvalidateAccount(av.GetProvider(), av.GetAccountId()); err != nil {
				m.log.Warn("derived-library invalidation failed; caches rebuild on next read", "error", err)
			}
		}
	}
}

// SlotRuntime is the composed provider-slot layer.
type SlotRuntime struct {
	// Bus carries sync-emitted events to subscribers.
	Bus *apiserver.InMemoryBus
	// Library derives and caches per-user libraries over every wired reach.
	Library *library.Service
	// ItemRegistry is the instance-wide provider item registry; exposed for
	// the future operator surface (merge-conflict review).
	ItemRegistry *itemregistry.Registry
	// Jobs are the slots' recurring refresh jobs; register them with a
	// scheduler and run it.
	Jobs []scheduler.Job
}

// ComposeSlots wires every enabled provider slot and the services that feed
// off them: the item registry (identity is instance-wide state over the
// source-cache store), the per-user library, and the event path between
// syncs and cache invalidation. reg is the caller-owned slot registry;
// sourceCache backs both the caches and the registry's mappings.
//
// No owner id goes into the item registry yet: operator-facing
// merge-conflict notifications arrive with the operator surface, until then
// the registry suppresses those envelopes.
func ComposeSlots(ctx context.Context, slots config.SlotsConfig, reg *registry.InProcessRegistry, sourceCache, vault store.Store, logger *slog.Logger) (*SlotRuntime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	itemReg, err := itemregistry.New(sourceCache, "")
	if err != nil {
		return nil, fmt.Errorf("item registry: %w", err)
	}

	rt := &SlotRuntime{Bus: apiserver.NewInMemoryBus()}
	mux := &eventMux{bus: rt.Bus, log: logger}

	jobs, reaches, err := slotwiring.SetupAll(ctx, slots, slotwiring.Deps{
		Ctx:          ctx,
		Registry:     reg,
		SealedBlobs:  NewSealedBlobs(vault),
		SourceCache:  sourceCache,
		Logger:       logger,
		ItemRegistry: itemReg,
		EventSink:    mux,
	})
	if err != nil {
		rt.Bus.Close()
		return nil, fmt.Errorf("slots: %w", err)
	}

	libSvc, err := library.NewService(reaches, itemReg, sourceCache, logger)
	if err != nil {
		rt.Bus.Close()
		return nil, fmt.Errorf("library: %w", err)
	}
	rt.Library, rt.ItemRegistry, rt.Jobs = libSvc, itemReg, jobs
	return rt, nil
}
