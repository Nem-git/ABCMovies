package slotwiring

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
	"github.com/nem-git/abcmovies/core/internal/config"
	"github.com/nem-git/abcmovies/core/internal/pacing"
	"github.com/nem-git/abcmovies/core/internal/sourcecache"
)

// pacedClient wraps one provider slot for one capability surface with the
// shared pacing machinery: every provider-bound call (catalogue sync, lazy
// refresh, produce-sources) passes the per-account Limiter and the
// slot-wide Governor through pacing.Gate. Background refresh and enrichment
// draw from the same per-account Budget as user traffic — background can
// never exceed the limits user requests obey (PLAN.md §7.2).
type pacedClient struct {
	innerCatalogue sourcecacheClient
	innerRefresh   sourcecache.RefreshClient
	innerProducer  produces
	byAccount      map[string]*pacing.Limiter
	budgetAccount  map[string]pacing.Budget
	governor       *pacing.Governor
}

type sourcecacheClient interface {
	CatalogueSync(ctx context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error)
}

type produces interface {
	ProduceSources(ctx context.Context, req *slotsv1.ProduceSourcesRequest) (*slotsv1.ProduceSourcesResponse, error)
}

// Compile-time guarantees: a paced client is usable as the sourcecache
// sync client / refresh client and carries the produce-sources surface.
var (
	_ sourcecache.Client        = (*pacedClient)(nil)
	_ sourcecache.RefreshClient = (*pacedClient)(nil)
)

// CatalogueSync gates through the account limiter, then the shared governor.
func (p *pacedClient) CatalogueSync(ctx context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error) {
	var resp *slotsv1.CatalogueSyncResponse
	err := pacing.Gate(ctx, p.limiter(req.GetAccountId()), p.governor, p.budget(req.GetAccountId()), func(ctx context.Context) error {
		var err error
		resp, err = p.innerCatalogue.CatalogueSync(ctx, req)
		return err
	})
	return resp, err
}

// RefreshAvailability gates the same way.
func (p *pacedClient) RefreshAvailability(ctx context.Context, req *slotsv1.RefreshAvailabilityRequest) (*slotsv1.RefreshAvailabilityResponse, error) {
	if p.innerRefresh == nil {
		return nil, fmt.Errorf("pacedClient: provider does not implement refresh availability")
	}
	var resp *slotsv1.RefreshAvailabilityResponse
	err := pacing.Gate(ctx, p.limiter(req.GetAccountId()), p.governor, p.budget(req.GetAccountId()), func(ctx context.Context) error {
		var err error
		resp, err = p.innerRefresh.RefreshAvailability(ctx, req)
		return err
	})
	return resp, err
}

// ProduceSources gates the same way.
func (p *pacedClient) ProduceSources(ctx context.Context, req *slotsv1.ProduceSourcesRequest) (*slotsv1.ProduceSourcesResponse, error) {
	var resp *slotsv1.ProduceSourcesResponse
	err := pacing.Gate(ctx, p.limiter(req.GetAccountId()), p.governor, p.budget(req.GetAccountId()), func(ctx context.Context) error {
		var err error
		resp, err = p.innerProducer.ProduceSources(ctx, req)
		return err
	})
	return resp, err
}

// HasRefresh reports whether the paced client has a real lazy-refresh
// implementation behind it (a provider without one refuses the lookup, it
// does not panic on a nil surface).
func (p *pacedClient) HasRefresh() bool { return p.innerRefresh != nil }

func (p *pacedClient) limiter(accountID string) *pacing.Limiter {
	if l, ok := p.byAccount[accountID]; ok {
		return l
	}
	l := pacing.NewLimiter(p.budgetAccount[accountID], nil)
	p.byAccount[accountID] = l
	return l
}

func (p *pacedClient) budget(accountID string) pacing.Budget {
	if b, ok := p.budgetAccount[accountID]; ok {
		return b
	}
	return pacing.Budget{}
}

// budgetFromPolicy maps the pacing vocabulary keys (TECHNICAL-DECISIONS.md
// §1.35) onto a Budget. All keys are optional; an empty map means off.
func budgetFromPolicy(m map[string]string) pacing.Budget {
	if len(m) == 0 {
		return pacing.Budget{}
	}
	var b pacing.Budget
	if v := m["pacingRequestsPerSecond"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			b.RequestsPerSec = f
		}
	}
	if v := m["pacingMaxConcurrentPulls"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			b.MaxConcurrent = n
		}
	}
	if v := m["pacingInterRequestDelay"]; v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			b.MinSpacing = d
		}
	}
	if v := m["pacingRetries"]; v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			b.MaxRetries = n
		}
	}
	return b
}

// governorFromOptions reads the per-provider aggregate keys from a slot
// entry's Options bag: pacingRequestsPerSecond bounds total requests/sec
// across all accounts of this slot (PLAN.md §7.2).
func governorFromOptions(entry config.SlotEntry) *pacing.Governor {
	var rps float64
	if v := entry.Options["pacingRequestsPerSecond"]; v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			rps = f
		}
	}
	return pacing.NewGovernor(rps, time.Second, 30*time.Second, nil)
}

// newPacedClient builds the shared slot pacing material: a governor per slot
// and a limiter+budget per operator-declared account (linked accounts get a
// zero budget until their own pacing vocabulary is linked in).
func newPacedClient(entry config.SlotEntry, innerCatalogue sourcecacheClient, innerRefresh sourcecache.RefreshClient, innerProduces produces) *pacedClient {
	if innerProduces == nil {
		return nil
	}
	pc := &pacedClient{
		innerCatalogue: innerCatalogue,
		innerRefresh:   innerRefresh,
		innerProducer:  innerProduces,
		byAccount:      map[string]*pacing.Limiter{},
		budgetAccount:  map[string]pacing.Budget{},
		governor:       governorFromOptions(entry),
	}
	for _, a := range entry.Accounts {
		pc.budgetAccount[a.ID] = budgetFromPolicy(a.Policy)
	}
	return pc
}

// fromSlot builds the pacing wrapper for a slot impl: a paced sourcecache
// sync/refresh client and a produces-wrapper for the delivery resolver.
// A nil pacedClient means the slot does not implement the lazy-refresh or
// catalogue-sync part — in practice every provider implements at least the
// catalogue side, so this is a defect rather than a supported path; callers
// should fail loudly if it ever trips.
func fromSlot(entry config.SlotEntry, inner any) (*pacedClient, produces) {
	syncer, ok := inner.(interface {
		CatalogueSync(ctx context.Context, req *slotsv1.CatalogueSyncRequest) (*slotsv1.CatalogueSyncResponse, error)
	})
	var refresh sourcecache.RefreshClient
	if r, ok2 := inner.(sourcecache.RefreshClient); ok2 {
		refresh = r
	}
	produce, ok3 := inner.(produces)
	if !ok || !ok3 || syncer == nil {
		return nil, nil
	}
	return newPacedClient(entry, syncer, refresh, produce), produce
}

// producesResolver bridges a pacedClient to the delivery.Resolver surface.
type producesResolver struct{ pc *pacedClient }

func (r producesResolver) ProduceSources(ctx context.Context, provider, accountID, nativeID string) (*corev1.MediaSource, error) {
	resp, err := r.pc.ProduceSources(ctx, &slotsv1.ProduceSourcesRequest{AccountId: accountID, NativeId: nativeID})
	if err != nil {
		return nil, err
	}
	return resp.GetSource(), nil
}
