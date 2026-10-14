package slotwiring

import (
	"fmt"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	"github.com/nem-git/abcmovies/core/internal/delivery"
)

const adapterMediaServer = "mediaserver"

func init() {
	RegisterSink(adapterMediaServer, buildMediaServer)
}

// buildMediaServer wires one media-server sink: a pull sink that exposes the
// session as an HLS manifest whose byte URLs point at the engine's relay. It
// needs both the relay (to grant tickets) and the opener (to produce clear,
// DRM-decrypted bytes for them). The adapter's manifest logic lives in
// adapters/mediaserver; this is the thin registration shim every built-in
// adapter has.
func buildMediaServer(_ *corev1.Slot, relay *delivery.Relay, opener *delivery.Opener) (delivery.SinkFactory, error) {
	if relay == nil {
		return nil, fmt.Errorf("media-server sink: no relay wired")
	}
	if opener == nil {
		return nil, fmt.Errorf("media-server sink: no opener wired")
	}
	return &delivery.MediaServerSinkFactory{Relay: relay, Opener: opener}, nil
}
