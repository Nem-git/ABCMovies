package handler_test

import (
	"testing"

	"github.com/nem-git/abcmovies/internal/handler"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/providers/stub"
	"github.com/nem-git/abcmovies/internal/registry"
)

func TestGetServices(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:     "test",
		Service: &oas.Service{Tag: "test", Name: "Test Service"},
	})
	r.Register(p)

	h := handler.New(r)
	res, err := h.GetServices(t.Context(), oas.GetServicesParams{
		Limit:  oas.NewOptInt(20),
		Offset: oas.NewOptInt(0),
	})
	if err != nil {
		t.Fatalf("GetServices() error: %v", err)
	}

	page, ok := res.(*oas.PageService)
	if !ok {
		t.Fatalf("GetServices() returned %T, want *oas.PageService", res)
	}
	if page.Total != 1 {
		t.Errorf("Total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 {
		t.Errorf("got %d items, want 1", len(page.Items))
	}
	if page.Items[0].GetTag() != "test" {
		t.Errorf("service tag = %q, want %q", page.Items[0].GetTag(), "test")
	}
}

func TestGetServicesMultipleProviders(t *testing.T) {
	r := registry.New()
	p1 := stub.New(stub.Config{
		Tag:     "a",
		Service: &oas.Service{Tag: "a", Name: "Service A"},
	})
	p2 := stub.New(stub.Config{
		Tag:     "b",
		Service: &oas.Service{Tag: "b", Name: "Service B"},
	})
	r.Register(p1)
	r.Register(p2)

	h := handler.New(r)
	res, err := h.GetServices(t.Context(), oas.GetServicesParams{
		Limit:  oas.NewOptInt(20),
		Offset: oas.NewOptInt(0),
	})
	if err != nil {
		t.Fatalf("GetServices() error: %v", err)
	}

	page := res.(*oas.PageService)
	if page.Total != 2 {
		t.Errorf("Total = %d, want 2", page.Total)
	}
	if len(page.Items) != 2 {
		t.Errorf("got %d items, want 2", len(page.Items))
	}
}

func TestGetServicesProviderError(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:   "broken",
		Error: provider.ErrNotSupported,
	})
	r.Register(p)

	h := handler.New(r)
	res, err := h.GetServices(t.Context(), oas.GetServicesParams{
		Limit:  oas.NewOptInt(20),
		Offset: oas.NewOptInt(0),
	})
	if err != nil {
		t.Fatalf("GetServices() error: %v", err)
	}

	errResp, ok := res.(*oas.ErrorStatusCode)
	if !ok {
		t.Fatalf("GetServices() returned %T, want *oas.ErrorStatusCode", res)
	}
	if errResp.StatusCode != 502 {
		t.Errorf("StatusCode = %d, want 502", errResp.StatusCode)
	}
	if errResp.Response.Code != "PROVIDER_ERROR" {
		t.Errorf("error code = %q, want %q", errResp.Response.Code, "PROVIDER_ERROR")
	}
}

func TestGetServicesEmpty(t *testing.T) {
	r := registry.New()
	h := handler.New(r)

	res, err := h.GetServices(t.Context(), oas.GetServicesParams{
		Limit:  oas.NewOptInt(20),
		Offset: oas.NewOptInt(0),
	})
	if err != nil {
		t.Fatalf("GetServices() error: %v", err)
	}

	page := res.(*oas.PageService)
	if page.Total != 0 {
		t.Errorf("Total = %d, want 0", page.Total)
	}
	if len(page.Items) != 0 {
		t.Errorf("got %d items, want 0", len(page.Items))
	}
}

func TestGetServicesPagination(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:     "test",
		Service: &oas.Service{Tag: "test", Name: "Test"},
	})
	r.Register(p)

	h := handler.New(r)

	t.Run("offset zero returns service", func(t *testing.T) {
		res, err := h.GetServices(t.Context(), oas.GetServicesParams{
			Limit:  oas.NewOptInt(20),
			Offset: oas.NewOptInt(0),
		})
		if err != nil {
			t.Fatalf("GetServices() error: %v", err)
		}
		page := res.(*oas.PageService)
		if len(page.Items) != 1 {
			t.Errorf("got %d items, want 1", len(page.Items))
		}
	})

	t.Run("offset past total returns error", func(t *testing.T) {
		res, err := h.GetServices(t.Context(), oas.GetServicesParams{
			Limit:  oas.NewOptInt(20),
			Offset: oas.NewOptInt(5),
		})
		if err != nil {
			t.Fatalf("GetServices() error: %v", err)
		}
		errResp, ok := res.(*oas.ErrorStatusCode)
		if !ok {
			t.Fatalf("GetServices() returned %T, want *oas.ErrorStatusCode", res)
		}
		if errResp.StatusCode != 400 {
			t.Errorf("StatusCode = %d, want 400", errResp.StatusCode)
		}
	})
}

func TestGetServiceByTag(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:     "test",
		Service: &oas.Service{Tag: "test", Name: "Test"},
	})
	r.Register(p)

	h := handler.New(r)

	t.Run("found", func(t *testing.T) {
		res, err := h.GetServiceByTag(t.Context(), oas.GetServiceByTagParams{ServiceTag: "test"})
		if err != nil {
			t.Fatalf("GetServiceByTag() error: %v", err)
		}
		svc, ok := res.(*oas.Service)
		if !ok {
			t.Fatalf("returned %T, want *oas.Service", res)
		}
		if svc.Tag != "test" {
			t.Errorf("Tag = %q, want %q", svc.Tag, "test")
		}
	})

	t.Run("not found", func(t *testing.T) {
		res, err := h.GetServiceByTag(t.Context(), oas.GetServiceByTagParams{ServiceTag: "unknown"})
		if err != nil {
			t.Fatalf("GetServiceByTag() error: %v", err)
		}
		errResp, ok := res.(*oas.ErrorStatusCode)
		if !ok {
			t.Fatalf("returned %T, want *oas.ErrorStatusCode", res)
		}
		if errResp.StatusCode != 404 {
			t.Errorf("StatusCode = %d, want 404", errResp.StatusCode)
		}
	})
}

func TestGetHealth(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		r := registry.New()
		p := stub.New(stub.Config{
			Tag:    "ok",
			Health: &oas.Health{Status: oas.HealthStatusOk},
		})
		r.Register(p)
		h := handler.New(r)

		health, err := h.GetHealth(t.Context())
		if err != nil {
			t.Fatalf("GetHealth() error: %v", err)
		}
		if health.Status != oas.HealthStatusOk {
			t.Errorf("Status = %q, want %q", health.Status, oas.HealthStatusOk)
		}
	})

	t.Run("degraded", func(t *testing.T) {
		r := registry.New()
		p := stub.New(stub.Config{
			Tag:   "broken",
			Error: provider.ErrNotSupported,
		})
		r.Register(p)
		h := handler.New(r)

		health, err := h.GetHealth(t.Context())
		if err != nil {
			t.Fatalf("GetHealth() error: %v", err)
		}
		if health.Status != oas.HealthStatusDegraded {
			t.Errorf("Status = %q, want %q", health.Status, oas.HealthStatusDegraded)
		}
	})

	t.Run("empty registry", func(t *testing.T) {
		h := handler.New(registry.New())
		health, err := h.GetHealth(t.Context())
		if err != nil {
			t.Fatalf("GetHealth() error: %v", err)
		}
		if health.Status != oas.HealthStatusOk {
			t.Errorf("Status = %q, want %q", health.Status, oas.HealthStatusOk)
		}
	})
}

func TestGlobalSearch(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag: "searchable",
		Search: []oas.SearchResultItem{
			{Score: 1, Resource: oas.SearchResultItemResource{Type: oas.MovieSearchResultItemResource, Movie: oas.Movie{ID: "m1"}}},
		},
	})
	r.Register(p)

	h := handler.New(r)

	t.Run("no results", func(t *testing.T) {
		r2 := registry.New()
		h2 := handler.New(r2)

		res, err := h2.GlobalSearch(t.Context(), oas.GlobalSearchParams{Q: "nothing"})
		if err != nil {
			t.Fatalf("GlobalSearch() error: %v", err)
		}
		page, ok := res.(*oas.PageSearchResult)
		if !ok {
			t.Fatalf("returned %T, want *oas.PageSearchResult", res)
		}
		if page.Total != 0 {
			t.Errorf("Total = %d, want 0", page.Total)
		}
	})

	t.Run("with results", func(t *testing.T) {
		res, err := h.GlobalSearch(t.Context(), oas.GlobalSearchParams{Q: "test", Limit: oas.NewOptInt(10), Offset: oas.NewOptInt(0)})
		if err != nil {
			t.Fatalf("GlobalSearch() error: %v", err)
		}
		page, ok := res.(*oas.PageSearchResult)
		if !ok {
			t.Fatalf("returned %T, want *oas.PageSearchResult", res)
		}
		if page.Total != 1 {
			t.Errorf("Total = %d, want 1", page.Total)
		}
	})
}

func TestProviderNotFound(t *testing.T) {
	h := handler.New(registry.New())

	t.Run("GetMovies", func(t *testing.T) {
		res, err := h.GetMovies(t.Context(), oas.GetMoviesParams{ServiceTag: "nope"})
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		errResp, ok := res.(*oas.ErrorStatusCode)
		if !ok {
			t.Fatalf("returned %T, want *oas.ErrorStatusCode", res)
		}
		if errResp.StatusCode != 404 {
			t.Errorf("StatusCode = %d, want 404", errResp.StatusCode)
		}
	})
}

func TestProviderError(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:   "broken",
		Error: provider.ErrNotSupported,
		Movies: []oas.Movie{
			{ID: "m1", Name: "M1"},
		},
	})
	r.Register(p)
	h := handler.New(r)

	t.Run("GetMovies", func(t *testing.T) {
		res, err := h.GetMovies(t.Context(), oas.GetMoviesParams{ServiceTag: "broken"})
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		errResp, ok := res.(*oas.ErrorStatusCode)
		if !ok {
			t.Fatalf("returned %T, want *oas.ErrorStatusCode", res)
		}
		if errResp.StatusCode != 502 {
			t.Errorf("StatusCode = %d, want 502", errResp.StatusCode)
		}
		if errResp.Response.Code != "PROVIDER_ERROR" {
			t.Errorf("Code = %q, want %q", errResp.Response.Code, "PROVIDER_ERROR")
		}
	})
}

func TestGetMovieById(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag: "test",
		Movies: []oas.Movie{
			{ID: "m1", Name: "Movie 1"},
		},
	})
	r.Register(p)
	h := handler.New(r)

	t.Run("found", func(t *testing.T) {
		res, err := h.GetMovieById(t.Context(), oas.GetMovieByIdParams{ServiceTag: "test", MovieId: "m1"})
		if err != nil {
			t.Fatalf("GetMovieById() error: %v", err)
		}
		movie, ok := res.(*oas.Movie)
		if !ok {
			t.Fatalf("returned %T, want *oas.Movie", res)
		}
		if movie.ID != "m1" {
			t.Errorf("ID = %q, want %q", movie.ID, "m1")
		}
	})

	t.Run("not found", func(t *testing.T) {
		res, err := h.GetMovieById(t.Context(), oas.GetMovieByIdParams{ServiceTag: "test", MovieId: "nope"})
		if err != nil {
			t.Fatalf("GetMovieById() error: %v", err)
		}
		errResp, ok := res.(*oas.ErrorStatusCode)
		if !ok {
			t.Fatalf("returned %T, want *oas.ErrorStatusCode", res)
		}
		if errResp.StatusCode != 502 {
			t.Errorf("StatusCode = %d, want 502", errResp.StatusCode)
		}
	})
}

func TestGetMovieStreams(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:     "test",
		Streams: []oas.Stream{{ID: "manifest.mpd", Name: "DASH", EncodingFormat: oas.StreamEncodingFormatApplicationDashXML}},
	})
	r.Register(p)
	h := handler.New(r)

	res, err := h.GetMovieStreams(t.Context(), oas.GetMovieStreamsParams{ServiceTag: "test", MovieId: "m1"})
	if err != nil {
		t.Fatalf("GetMovieStreams() error: %v", err)
	}
	page, ok := res.(*oas.PageStream)
	if !ok {
		t.Fatalf("returned %T, want *oas.PageStream", res)
	}
	if page.Total != 1 {
		t.Errorf("Total = %d, want 1", page.Total)
	}
	if len(page.Items) != 1 {
		t.Errorf("got %d items, want 1", len(page.Items))
	}
}

func TestGetMoviePoster(t *testing.T) {
	r := registry.New()
	p := stub.New(stub.Config{
		Tag:             "test",
		MoviePosterData: []byte("fake-png"),
	})
	r.Register(p)
	h := handler.New(r)

	res, err := h.GetMoviePoster(t.Context(), oas.GetMoviePosterParams{ServiceTag: "test", MovieId: "m1"})
	if err != nil {
		t.Fatalf("GetMoviePoster() error: %v", err)
	}
	_, ok := res.(*oas.GetMoviePosterOK)
	if !ok {
		t.Fatalf("returned %T, want *oas.GetMoviePosterOK", res)
	}
}
