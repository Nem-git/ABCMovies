package app

import (
	"context"
	"fmt"
	"log/slog"

	apiv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/api/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/apiserver"
	"github.com/nem-git/abcmovies/core/internal/library"
	"github.com/nem-git/abcmovies/core/internal/slotwiring"
)

// liveSearcher is the composition root for LiveSearch: the explicit,
// user-triggered provider refresh (PLAN.md §5.4). It walks the built provider
// slots, runs each lazy one's SearchCatalog, and feeds the returned pages
// through the same synchronizer that owns the source cache for that account
// — the same upsert/resolve/availability-event path a catalogue sync page
// would use, under pacing via the slot's paced client.
type liveSearcher struct {
	built   []*slotwiring.BuiltSlot
	library *library.Service
	log     *slog.Logger
}

func (l *liveSearcher) Run(ctx context.Context, req *apiv1.LiveSearchRequest) (*apiv1.LiveSearchResponse, error) {
	uid, _ := apiserver.UserIDFromContext(ctx)
	allowed := map[string]bool{}
	for _, r := range l.library.ReachesForUser(uid) {
		allowed[r.AccountID] = true
	}
	pageLimit := int(req.GetPageLimit())
	if pageLimit <= 0 {
		pageLimit = 2
	}
	if pageLimit > 5 {
		pageLimit = 5
	}

	hits := []*apiv1.LiveSearchHit{}
	rejected := map[string]string{}
	for _, b := range l.built {
		if b.LazySearch == nil {
			continue // whole-catalogue providers only sync on cadence
		}
		if len(req.GetProviders()) > 0 && !slotListed(b.Entry.ID, req.GetProviders()) {
			continue
		}
		ok := true
		for _, reach := range b.Reaches {
			if !allowed[reach.AccountID] {
				continue
			}
			token := ""
			for page := 0; page < pageLimit; page++ {
				resp, err := b.LazySearch.SearchCatalog(ctx, &slotsv1.SearchCatalogRequest{
					AccountId: reach.AccountID, Query: req.GetQuery(), PageToken: token,
				})
				if err != nil {
					rejected[b.Entry.ID] = fmt.Sprintf("%s/%s: %v", b.Entry.ID, reach.AccountID, err)
					ok = false
					break
				}
				if _, err := reach.Sync.IngestItems(ctx, reach.AccountID, resp.GetItems()); err != nil {
					rejected[b.Entry.ID] = fmt.Sprintf("ingest %s/%s: %v", b.Entry.ID, reach.AccountID, err)
					ok = false
					break
				}
				for _, it := range resp.GetItems() {
					hits = append(hits, &apiv1.LiveSearchHit{
						Provider:  b.Entry.ID,
						AccountId: reach.AccountID,
						NativeId:  it.GetNativeId(),
						Title:     it.GetMetadata().GetTitle(),
						Kind:      it.GetKind().String(),
						Year:      it.GetMetadata().GetYear(),
					})
				}
				if token = resp.GetNextPageToken(); token == "" {
					break
				}
			}
			if !ok {
				break // one account error on this slot: keep the slot's other accounts out too
			}
		}
	}
	return &apiv1.LiveSearchResponse{Hits: hits, Rejected: rejected}, nil
}

func slotListed(id string, list []string) bool {
	for _, x := range list {
		if x == id {
			return true
		}
	}
	return false
}

func liveSlotsFromBuilt(built []*slotwiring.BuiltSlot, lib *library.Service, log *slog.Logger) apiserver.LiveSearcher {
	if log == nil {
		log = slog.Default()
	}
	return &liveSearcher{built: built, library: lib, log: log}
}
