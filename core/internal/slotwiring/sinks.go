package slotwiring

import (
	"context"
	"fmt"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

const (
	adapterDisk  = "disk"
	adapterRelay = "relay"
)

// compositeSinkFactory dispatches a delivery session to the concrete sink
// factory named by its Sink, per PLAN.md §6.4 (sinks are declared in instance
// config). An unknown sink name is a loud error, never a silent drop.
type compositeSinkFactory struct {
	factories map[string]delivery.SinkFactory
}

func (c *compositeSinkFactory) NewSink(ctx context.Context, s *delivery.Session, tracks []*corev1.Track) (delivery.Sink, error) {
	name := s.Context.GetSink()
	f, ok := c.factories[name]
	if !ok {
		return nil, fmt.Errorf("no configured sink named %q", name)
	}
	return f.NewSink(ctx, s, tracks)
}

// SinkBuilder builds one sink factory for a slot, given the shared byte
// machinery every sink needs: the relay (pull sinks) and the opener (DRM-aware
// sinks). It is the registration seam — an adapter registers itself by name and
// the core looks it up, so adding a sink type needs no core change (PLAN.md
// §6.4: sinks are added without touching the core).
type SinkBuilder func(slot *corev1.Slot, relay *delivery.Relay, opener *delivery.Opener) (delivery.SinkFactory, error)

var sinkBuilders = map[string]SinkBuilder{}

// RegisterSink wires a sink adapter implementation to its config name. Called
// from each adapter's wiring file via init() — the same pattern providers,
// catalogues, and probers use.
func RegisterSink(name string, b SinkBuilder) {
	if _, dup := sinkBuilders[name]; dup {
		panic(fmt.Sprintf("slotwiring: sink adapter %q registered twice", name))
	}
	sinkBuilders[name] = b
}

// SetupSinks wires the enabled sink slots into one composite factory over a
// shared relay. v1 ships two co-equal sinks (TECHNICAL-DECISIONS.md §1.13):
// the instance-local disk (declared; must name a path) and the relay pull
// sink (built-in; stages per-track relay tickets for any conformant player —
// a user's device or a media server — and never knows about packaging). Each
// enabled slot is reachable by its slot id. Zero enabled sinks yields a
// factory that rejects every start.
func SetupSinks(slots []*corev1.Slot, relay *delivery.Relay, opener *delivery.Opener) (delivery.SinkFactory, error) {
	composite := &compositeSinkFactory{factories: map[string]delivery.SinkFactory{}}
	for _, slot := range slots {
		if slot.GetKind() != corev1.SlotKind_SLOT_KIND_SINK {
			continue
		}
		sinkCfg := slot.GetSink()
		build, ok := sinkBuilders[sinkCfg.GetAdapter()]
		if !ok {
			return nil, fmt.Errorf("sink %q: unknown adapter %q (registered: %s)", slot.GetId(), sinkCfg.GetAdapter(), keys(sinkBuilders))
		}
		f, err := build(slot, relay, opener)
		if err != nil {
			return nil, fmt.Errorf("sink %q (%s): %w", slot.GetId(), sinkCfg.GetAdapter(), err)
		}
		composite.factories[slot.GetId()] = f
	}
	return composite, nil
}

// The two co-equal v1 sinks register here: the instance-local disk (declared;
// must name a path) and the relay pull sink (built-in; stages per-track relay
// tickets for any conformant player).
func init() {
	RegisterSink(adapterDisk, buildDisk)
	RegisterSink(adapterRelay, buildRelay)
}

func buildDisk(slot *corev1.Slot, _ *delivery.Relay, _ *delivery.Opener) (delivery.SinkFactory, error) {
	path := slot.GetSink().GetOptions()["path"]
	if path == "" {
		return nil, fmt.Errorf("options.path is required")
	}
	return delivery.NewDiskFactory(path)
}

func buildRelay(_ *corev1.Slot, relay *delivery.Relay, opener *delivery.Opener) (delivery.SinkFactory, error) {
	if relay == nil {
		return nil, fmt.Errorf("no relay wired")
	}
	return &delivery.RelaySinkFactory{Relay: relay, Opener: opener}, nil
}
