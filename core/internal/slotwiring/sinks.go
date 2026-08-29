package slotwiring

import (
	"context"
	"fmt"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
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

// SetupSinks wires the enabled sink entries into one composite factory over a
// shared relay. v1 ships two co-equal sinks (TECHNICAL-DECISIONS.md §1.13):
// the instance-local disk (declared; must name a path) and the user's device
// (built-in; its config is the frontend's). Each enabled entry is reachable by
// its slot id. Zero enabled sinks yields a factory that rejects every start.
func SetupSinks(entries []config.SlotEntry, relay *delivery.Relay) (delivery.SinkFactory, error) {
	composite := &compositeSinkFactory{factories: map[string]delivery.SinkFactory{}}
	for _, entry := range entries {
		if !entry.Enabled {
			continue
		}
		var f delivery.SinkFactory
		switch entry.Adapter {
		case adapterDisk:
			path := entry.Options["path"]
			if path == "" {
				return nil, fmt.Errorf("sink %q (disk): options.path is required", entry.ID)
			}
			disk, err := delivery.NewDiskFactory(path)
			if err != nil {
				return nil, fmt.Errorf("sink %q (disk): %w", entry.ID, err)
			}
			f = disk
		case adapterDevice:
			if relay == nil {
				return nil, fmt.Errorf("sink %q (device): no relay wired", entry.ID)
			}
			f = &delivery.DeviceSinkFactory{Relay: relay}
		default:
			return nil, fmt.Errorf("sink %q: unknown adapter %q (registered: disk, device)", entry.ID, entry.Adapter)
		}
		composite.factories[entry.ID] = f
	}
	return composite, nil
}
