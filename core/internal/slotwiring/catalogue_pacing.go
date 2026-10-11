package slotwiring

import (
	"context"
	"sync"
	"time"

	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/enrichment"
)

// cataloguePaceKey is the handshake policy key through which a catalogue
// adapter declares how fast it wants to be called. It rides the same channel
// as the provider cadence keys (the CapabilityQuery policy map), so the core
// never branches on it: the adapter declares, wiring reads, and an operator
// can override per slot in that slot's options. Additive in v1 — unknown keys
// are ignored (meta.proto).
const cataloguePaceKey = "catalogue.pace-interval"

// defaultCataloguePace is the shipped fallback when neither the operator nor
// the adapter declares a pace: the serial enrichment worker's historical
// spacing, which keeps a ~5 req/s ceiling on catalogue calls
// (TECHNICAL-DECISIONS §1.27). It stays the default so declaring nothing is
// behaviour-preserving.
const defaultCataloguePace = 200 * time.Millisecond

// pacedCatalogue spaces one catalogue's calls by the catalogue's declared
// pace. It is the enforcement point for "the thing that protects the provider
// declares how it wants to be protected": each catalogue carries its own
// interval, so two catalogues with different politeness no longer share one
// constant. The mutex serializes calls through this catalogue — the drain
// worker is already serial, and a second caller must not double the rate.
type pacedCatalogue struct {
	inner enrichment.CatalogueClient
	pace  time.Duration

	mu sync.Mutex
}

var _ enrichment.CatalogueClient = (*pacedCatalogue)(nil)

// wait blocks until this catalogue's next call is due.
func (p *pacedCatalogue) wait() {
	p.mu.Lock()
	defer p.mu.Unlock()
	time.Sleep(p.pace)
}

func (p *pacedCatalogue) LookupTitle(ctx context.Context, req *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
	p.wait()
	return p.inner.LookupTitle(ctx, req)
}

func (p *pacedCatalogue) GetMetadata(ctx context.Context, req *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
	p.wait()
	return p.inner.GetMetadata(ctx, req)
}
