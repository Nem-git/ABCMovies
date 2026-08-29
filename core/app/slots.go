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
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/delivery"
	"github.com/nem-git/abcmovies/core/internal/enrichment"
	"github.com/nem-git/abcmovies/core/internal/itemregistry"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/metadatacache"
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
	// Queue is the enrichment backlog; entries land here from the T1/T2
	// triggers and leave through the drain worker.
	Queue *enrichment.InMemoryQueue
	// Meta is the metadata cache enriched records land in; exposed for the
	// observability surface.
	Meta *metadatacache.Cache
	// Engine is the enrichment pipeline the worker drains the queue through.
	Engine *enrichment.Engine
	// Catalogues are the enabled catalogue slots the engine consults; exposed
	// for the observability surface so the operator sees which providers a
	// record could have been enriched from.
	Catalogues []enrichment.Catalogue
	// Jobs are the slots' recurring refresh jobs; register them with a
	// scheduler and run it. The enrichment drain job is included.
	Jobs []scheduler.Job

	// Delivery pieces, set when the delivery engine is composed: Resolvers
	// maps a provider slot id to its produce-sources resolver; Sinks is the
	// composite sink factory built from the configured sinks; Relay grants
	// session-scoped media access (PLAN.md §3.6). The engine itself is built
	// and armed by Stack.BuildSlots, which owns the jobs store and lifecycle.
	Resolvers slotwiring.Resolvers
	Sinks     delivery.SinkFactory
	Relay     *delivery.Relay
}

// registryEvidence adapts the item registry to the enrichment engine's
// EntrySource: an entry's evidence is exactly its stored identity proof —
// kind, asserted external IDs, title and year — the material matching
// already trusts (PLAN.md §5.3).
type registryEvidence struct{ r *itemregistry.Registry }

func (e registryEvidence) Evidence(ctx context.Context, entryID string) (enrichment.EntryEvidence, bool, error) {
	canon, ok, err := e.r.Canonical(ctx, entryID)
	if err != nil || !ok {
		return enrichment.EntryEvidence{}, false, err
	}
	ids := make([]*slotsv1.ExternalId, 0, len(canon.Claims))
	for _, c := range canon.Claims {
		ids = append(ids, &slotsv1.ExternalId{Namespace: c.Namespace, Value: c.Value})
	}
	return enrichment.EntryEvidence{
		Kind:        canon.Kind,
		Metadata:    &corev1.TitleMetadata{Title: canon.Title, Year: canon.Year},
		ExternalIDs: ids,
	}, true, nil
}

// ComposeSlots wires every enabled provider slot and the services that feed
// off them: the item registry (identity is instance-wide state over the
// source-cache store), the per-user library, the enrichment pipeline
// (queue, engine, drain worker), and the event path between syncs and cache
// invalidation. reg is the caller-owned slot registry; sourceCache backs
// both the caches and the registry's mappings; metaCache holds enriched
// records.
//
// No owner id goes into the item registry yet: operator-facing
// merge-conflict notifications arrive with the operator surface, until then
// the registry suppresses those envelopes.
func ComposeSlots(ctx context.Context, slots config.SlotsConfig, enrich config.EnrichmentConfig, reg *registry.InProcessRegistry, sourceCache, metaCache, vault store.Store, logger *slog.Logger) (*SlotRuntime, error) {
	if logger == nil {
		logger = slog.Default()
	}
	itemReg, err := itemregistry.New(sourceCache, "")
	if err != nil {
		return nil, fmt.Errorf("item registry: %w", err)
	}
	meta, err := metadatacache.New(metaCache, logger)
	if err != nil {
		return nil, fmt.Errorf("metadata cache: %w", err)
	}
	queue := enrichment.NewInMemoryQueue()

	rt := &SlotRuntime{Bus: apiserver.NewInMemoryBus(), Queue: queue}
	mux := &eventMux{bus: rt.Bus, log: logger}

	rt.Relay = delivery.NewRelay()
	jobs, reaches, cats, resolvers, err := slotwiring.SetupAll(ctx, slots, slotwiring.Deps{
		Ctx:          ctx,
		Registry:     reg,
		SealedBlobs:  NewSealedBlobs(vault),
		SourceCache:  sourceCache,
		Logger:       logger,
		ItemRegistry: itemReg,
		EventSink:    mux,
		Enqueue:      queue.Enqueue,
	})
	if err != nil {
		rt.Bus.Close()
		return nil, fmt.Errorf("slots: %w", err)
	}
	rt.Resolvers = resolvers

	srvs, err := slotwiring.SetupSinks(slots.Sinks, rt.Relay)
	if err != nil {
		rt.Bus.Close()
		return nil, fmt.Errorf("sinks: %w", err)
	}
	rt.Sinks = srvs

	// The enrichment pipeline drains whatever the T1/T2 triggers collect;
	// with no catalogue slots enabled the queue simply stays empty.
	engine := enrichment.NewEngine(registryEvidence{r: itemReg}, meta, cats, logger)
	drainCadence, err := enrichment.DrainCadence(enrich.DrainCadence)
	if err != nil {
		rt.Bus.Close()
		return nil, fmt.Errorf("enrichment: %w", err)
	}
	worker := enrichment.NewWorker(queue, engine.Enrich, logger)
	jobs = append(jobs, worker.Job(drainCadence))

	libSvc, err := library.NewService(reaches, itemReg, sourceCache, logger,
		library.WithEnrichment(meta, queue.Enqueue))
	if err != nil {
		rt.Bus.Close()
		return nil, fmt.Errorf("library: %w", err)
	}
	rt.Library, rt.ItemRegistry, rt.Jobs = libSvc, itemReg, jobs
	rt.Meta = meta
	rt.Engine = engine
	rt.Catalogues = cats
	return rt, nil
}
