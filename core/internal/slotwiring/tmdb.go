package slotwiring

import (
	"context"
	"fmt"

	"github.com/nem-git/abcmovies/adapters/tmdb"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/enrichment"
)

func init() {
	RegisterCatalogue("tmdb", wireTMDB)
}

// wireTMDB builds one TMDB catalogue instance. The credential travels via
// the slot's token-env (TECHNICAL-DECISIONS.md §1.27): the adapter reads
// the named environment variable at composition time and startup fails
// loudly when it is absent — a half-credentialed catalogue would silently
// enrich nothing. The slot is returned unpublished; the composition root
// admits it as its final step.
func wireTMDB(entry config.SlotEntry, deps Deps) (*builtCatalogue, error) {
	if entry.TokenEnv == "" {
		return nil, fmt.Errorf("slot %q: catalogue slots deliver their credential through token-env; none configured", entry.ID)
	}
	slot, err := tmdb.New(entry.TokenEnv)
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	return &builtCatalogue{
		catalogue: enrichment.Catalogue{
			Slot:   entry.ID,
			Client: &catalogClient{slot: slot},
		},
		impl: slot,
	}, nil
}

// catalogClient narrows the adapter to exactly the engine's contract
// surface, so the engine can never grow dependencies on adapter internals.
type catalogClient struct {
	slot *tmdb.Slot
}

func (c *catalogClient) LookupTitle(ctx context.Context, req *slotsv1.LookupTitleRequest) (*slotsv1.LookupTitleResponse, error) {
	return c.slot.LookupTitle(ctx, req)
}

func (c *catalogClient) GetMetadata(ctx context.Context, req *slotsv1.GetMetadataRequest) (*slotsv1.GetMetadataResponse, error) {
	return c.slot.GetMetadata(ctx, req)
}
