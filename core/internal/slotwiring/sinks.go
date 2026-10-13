package slotwiring

import (
	"context"
	"fmt"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

const (
	adapterDisk   = "disk"
	adapterDevice = "device"
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

// SetupSinks wires the enabled sink slots into one composite factory over a
// shared relay. v1 ships two co-equal sinks (TECHNICAL-DECISIONS.md §1.13):
// the instance-local disk (declared; must name a path) and the user's device
// (built-in; its config is the frontend's). Each enabled slot is reachable by
// its slot id. Zero enabled sinks yields a factory that rejects every start.
func SetupSinks(slots []*corev1.Slot, relay *delivery.Relay) (delivery.SinkFactory, error) {
	composite := &compositeSinkFactory{factories: map[string]delivery.SinkFactory{}}
	for _, slot := range slots {
		if slot.GetKind() != corev1.SlotKind_SLOT_KIND_SINK {
			continue
		}
		sinkCfg := slot.GetSink()
		var f delivery.SinkFactory
		switch sinkCfg.GetAdapter() {
		case adapterDisk:
			path := sinkCfg.GetOptions()["path"]
			if path == "" {
				return nil, fmt.Errorf("sink %q (disk): options.path is required", slot.GetId())
			}
			disk, err := delivery.NewDiskFactory(path)
			if err != nil {
				return nil, fmt.Errorf("sink %q (disk): %w", slot.GetId(), err)
			}
			f = disk
		case adapterDevice:
			if relay == nil {
				return nil, fmt.Errorf("sink %q (device): no relay wired", slot.GetId())
			}
			f = &delivery.DeviceSinkFactory{Relay: relay}
		default:
			return nil, fmt.Errorf("sink %q: unknown adapter %q (registered: disk, device)", slot.GetId(), sinkCfg.GetAdapter())
		}
		composite.factories[slot.GetId()] = f
	}
	return composite, nil
}
