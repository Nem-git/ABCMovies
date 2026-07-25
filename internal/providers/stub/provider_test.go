package stub_test

import (
	"io"
	"testing"

	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/provider"
	"github.com/nem-git/abcmovies/internal/providers/stub"
)

func TestTag(t *testing.T) {
	p := stub.New(stub.Config{Tag: "test"})
	if got := p.Tag(); got != "test" {
		t.Errorf("Tag() = %q, want %q", got, "test")
	}
}

func TestPaginate(t *testing.T) {
	items := []string{"A", "B", "C", "D"}

	tests := []struct {
		name   string
		items  []string
		limit  int
		offset int
		wantN  int
		want   []string
	}{
		{name: "empty slice", items: nil, limit: 10, offset: 0, wantN: 0, want: nil},
		{name: "limit > n", items: items, limit: 10, offset: 0, wantN: 4, want: []string{"A", "B", "C", "D"}},
		{name: "limit = n", items: items, limit: 4, offset: 0, wantN: 4, want: []string{"A", "B", "C", "D"}},
		{name: "first page", items: items, limit: 2, offset: 0, wantN: 4, want: []string{"A", "B"}},
		{name: "mid-list page", items: items, limit: 2, offset: 1, wantN: 4, want: []string{"B", "C"}},
		{name: "last partial page", items: items, limit: 2, offset: 3, wantN: 4, want: []string{"D"}},
		{name: "offset past end", items: items, limit: 10, offset: 10, wantN: 4, want: nil},
		{name: "offset at exactly end", items: items, limit: 10, offset: 4, wantN: 4, want: nil},
		{name: "limit = 0", items: items, limit: 0, offset: 0, wantN: 4, want: nil},
		{name: "limit negative", items: items, limit: -1, offset: 0, wantN: 4, want: nil},
		{name: "single item first", items: []string{"X"}, limit: 1, offset: 0, wantN: 1, want: []string{"X"}},
		{name: "single item past end", items: []string{"X"}, limit: 10, offset: 5, wantN: 1, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, n := stub.Paginate(tt.items, tt.limit, tt.offset)
			if n != tt.wantN {
				t.Errorf("total = %d, want %d", n, tt.wantN)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d items, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("item[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestService(t *testing.T) {
	t.Run("configured", func(t *testing.T) {
		srv := &oas.Service{Tag: "test", Name: "Test Service"}
		p := stub.New(stub.Config{Service: srv})
		got, err := p.Service(t.Context())
		if err != nil {
			t.Fatalf("Service() error: %v", err)
		}
		if got != srv {
			t.Errorf("Service() returned different pointer")
		}
	})

	t.Run("not configured", func(t *testing.T) {
		p := stub.New(stub.Config{Tag: "test"})
		_, err := p.Service(t.Context())
		if err != provider.ErrNotSupported {
			t.Errorf("Service() error = %v, want ErrNotSupported", err)
		}
	})
}

func TestHealth(t *testing.T) {
	t.Run("configured", func(t *testing.T) {
		h := &oas.Health{Status: oas.HealthStatusOk}
		p := stub.New(stub.Config{Health: h})
		got, err := p.Health(t.Context())
		if err != nil {
			t.Fatalf("Health() error: %v", err)
		}
		if got != h {
			t.Errorf("Health() returned different pointer")
		}
	})

	t.Run("not configured", func(t *testing.T) {
		p := stub.New(stub.Config{Tag: "test"})
		_, err := p.Health(t.Context())
		if err != provider.ErrNotSupported {
			t.Errorf("Health() error = %v, want ErrNotSupported", err)
		}
	})
}

func TestGlobalError(t *testing.T) {
	errSentinel := provider.ErrNotSupported
	p := stub.New(stub.Config{Tag: "test", Error: errSentinel})

	t.Run("Service", func(t *testing.T) {
		_, err := p.Service(t.Context())
		if err != errSentinel {
			t.Errorf("Service() error = %v, want %v", err, errSentinel)
		}
	})

	t.Run("GetMovies", func(t *testing.T) {
		_, _, err := p.GetMovies(t.Context(), 10, 0)
		if err != errSentinel {
			t.Errorf("GetMovies() error = %v, want %v", err, errSentinel)
		}
	})

	t.Run("GetMovieById", func(t *testing.T) {
		_, err := p.GetMovieById(t.Context(), "id")
		if err != errSentinel {
			t.Errorf("GetMovieById() error = %v, want %v", err, errSentinel)
		}
	})

	t.Run("Search", func(t *testing.T) {
		_, _, err := p.Search(t.Context(), "q", 10, 0)
		if err != errSentinel {
			t.Errorf("Search() error = %v, want %v", err, errSentinel)
		}
	})
}

func TestMovies(t *testing.T) {
	movies := []oas.Movie{
		{Type: oas.MovieTypeMovie, ID: "m1", Name: "Movie 1"},
		{Type: oas.MovieTypeMovie, ID: "m2", Name: "Movie 2"},
		{Type: oas.MovieTypeMovie, ID: "m3", Name: "Movie 3"},
	}
	p := stub.New(stub.Config{Movies: movies})

	t.Run("GetMovies returns all", func(t *testing.T) {
		got, total, err := p.GetMovies(t.Context(), 10, 0)
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(got) != 3 {
			t.Errorf("got %d items, want 3", len(got))
		}
	})

	t.Run("GetMovies pagination", func(t *testing.T) {
		got, total, err := p.GetMovies(t.Context(), 2, 1)
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(got) != 2 {
			t.Errorf("got %d items, want 2", len(got))
		}
		if got[0].GetID() != "m2" {
			t.Errorf("first item ID = %q, want %q", got[0].GetID(), "m2")
		}
	})

	t.Run("GetMovies offset past end", func(t *testing.T) {
		got, total, err := p.GetMovies(t.Context(), 10, 10)
		if err != nil {
			t.Fatalf("GetMovies() error: %v", err)
		}
		if total != 3 {
			t.Errorf("total = %d, want 3", total)
		}
		if len(got) != 0 {
			t.Errorf("got %d items, want 0", len(got))
		}
	})

	t.Run("GetMovieById found", func(t *testing.T) {
		got, err := p.GetMovieById(t.Context(), "m2")
		if err != nil {
			t.Fatalf("GetMovieById() error: %v", err)
		}
		if got.GetName() != "Movie 2" {
			t.Errorf("name = %q, want %q", got.GetName(), "Movie 2")
		}
	})

	t.Run("GetMovieById not found", func(t *testing.T) {
		_, err := p.GetMovieById(t.Context(), "nonexistent")
		if err != provider.ErrNotSupported {
			t.Errorf("error = %v, want ErrNotSupported", err)
		}
	})
}

func TestSeries(t *testing.T) {
	series := []oas.Series{
		{Type: oas.SeriesTypeTVSeries, ID: "s1", Name: "Series 1"},
		{Type: oas.SeriesTypeTVSeries, ID: "s2", Name: "Series 2"},
	}
	p := stub.New(stub.Config{Series: series})

	t.Run("GetSeries", func(t *testing.T) {
		got, total, err := p.GetSeries(t.Context(), 10, 0)
		if err != nil {
			t.Fatalf("GetSeries() error: %v", err)
		}
		if total != 2 {
			t.Errorf("total = %d, want 2", total)
		}
		if len(got) != 2 {
			t.Errorf("got %d items, want 2", len(got))
		}
	})

	t.Run("GetSeriesByID found", func(t *testing.T) {
		got, err := p.GetSeriesByID(t.Context(), "s1")
		if err != nil {
			t.Fatalf("GetSeriesByID() error: %v", err)
		}
		if got.GetName() != "Series 1" {
			t.Errorf("name = %q, want %q", got.GetName(), "Series 1")
		}
	})
}

func TestSeasons(t *testing.T) {
	seasons := []oas.Season{
		{Type: oas.SeasonTypeTVSeason, ID: "sea1", Name: "Season 1"},
		{Type: oas.SeasonTypeTVSeason, ID: "sea2", Name: "Season 2"},
	}
	p := stub.New(stub.Config{Seasons: seasons})

	t.Run("GetSeasons", func(t *testing.T) {
		got, total, err := p.GetSeasons(t.Context(), "any-series", 10, 0)
		if err != nil {
			t.Fatalf("GetSeasons() error: %v", err)
		}
		if total != 2 {
			t.Errorf("total = %d, want 2", total)
		}
		if len(got) != 2 {
			t.Errorf("got %d items, want 2", len(got))
		}
	})

	t.Run("GetSeasonById ignores seriesID", func(t *testing.T) {
		got, err := p.GetSeasonById(t.Context(), "any-series", "sea1")
		if err != nil {
			t.Fatalf("GetSeasonById() error: %v", err)
		}
		if got.GetName() != "Season 1" {
			t.Errorf("name = %q, want %q", got.GetName(), "Season 1")
		}
	})
}

func TestEpisodes(t *testing.T) {
	episodes := []oas.Episode{
		{Type: oas.EpisodeTypeTVEpisode, ID: "ep1", Name: "Episode 1"},
	}
	p := stub.New(stub.Config{Episodes: episodes})

	t.Run("GetEpisodes", func(t *testing.T) {
		_, total, err := p.GetEpisodes(t.Context(), "s", "sea", 10, 0)
		if err != nil {
			t.Fatalf("GetEpisodes() error: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
	})

	t.Run("GetEpisodeById", func(t *testing.T) {
		got, err := p.GetEpisodeById(t.Context(), "s", "sea", "ep1")
		if err != nil {
			t.Fatalf("GetEpisodeById() error: %v", err)
		}
		if got.GetName() != "Episode 1" {
			t.Errorf("name = %q, want %q", got.GetName(), "Episode 1")
		}
	})
}

func TestStreamsAndSubtitles(t *testing.T) {
	streams := []oas.Stream{
		{Type: oas.StreamTypeVideoObject, ID: "manifest.mpd", Name: "DASH"},
	}
	subs := []oas.Subtitle{
		{ID: "en.vtt", Language: "en", Format: oas.SubtitleFormatVtt},
	}
	p := stub.New(stub.Config{Streams: streams, Subtitles: subs})

	t.Run("GetMovieStreams", func(t *testing.T) {
		got, total, err := p.GetMovieStreams(t.Context(), "any")
		if err != nil {
			t.Fatalf("GetMovieStreams() error: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
		if len(got) != 1 {
			t.Errorf("got %d items, want 1", len(got))
		}
	})

	t.Run("GetEpisodeStreams", func(t *testing.T) {
		got, total, err := p.GetEpisodeStreams(t.Context(), "s", "sea", "ep")
		if err != nil {
			t.Fatalf("GetEpisodeStreams() error: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
		if len(got) != 1 {
			t.Errorf("got %d items, want 1", len(got))
		}
	})

	t.Run("GetMovieSubtitles", func(t *testing.T) {
		_, total, err := p.GetMovieSubtitles(t.Context(), "any")
		if err != nil {
			t.Fatalf("GetMovieSubtitles() error: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
	})

	t.Run("GetEpisodeSubtitles", func(t *testing.T) {
		_, total, err := p.GetEpisodeSubtitles(t.Context(), "s", "sea", "ep")
		if err != nil {
			t.Fatalf("GetEpisodeSubtitles() error: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
	})
}

func TestImageEndpoints(t *testing.T) {
	data := []byte("fake-image-bytes")
	p := stub.New(stub.Config{
		MoviePosterData:      data,
		MovieBackdropData:    data,
		SeriesPosterData:     data,
		SeriesBackdropData:   data,
		SeasonPosterData:     data,
		SeasonBackdropData:   data,
		EpisodeThumbnailData: data,
		ImageMIME:            "image/png",
	})

	checkImage := func(name string, fn func() (io.ReadCloser, string, error)) {
		t.Helper()
		rc, mime, err := fn()
		if err != nil {
			t.Fatalf("%s() error: %v", name, err)
		}
		if mime != "image/png" {
			t.Errorf("%s() mime = %q, want %q", name, mime, "image/png")
		}
		got, _ := io.ReadAll(rc)
		rc.Close()
		if string(got) != "fake-image-bytes" {
			t.Errorf("%s() = %q, want %q", name, string(got), "fake-image-bytes")
		}
	}

	checkImage("GetMoviePoster", func() (io.ReadCloser, string, error) { return p.GetMoviePoster(t.Context(), "m1") })
	checkImage("GetMovieBackdrop", func() (io.ReadCloser, string, error) { return p.GetMovieBackdrop(t.Context(), "m1") })
	checkImage("GetSeriesPoster", func() (io.ReadCloser, string, error) { return p.GetSeriesPoster(t.Context(), "s1") })
	checkImage("GetSeriesBackdrop", func() (io.ReadCloser, string, error) { return p.GetSeriesBackdrop(t.Context(), "s1") })
	checkImage("GetSeasonPoster", func() (io.ReadCloser, string, error) { return p.GetSeasonPoster(t.Context(), "s1", "sea1") })
	checkImage("GetSeasonBackdrop", func() (io.ReadCloser, string, error) { return p.GetSeasonBackdrop(t.Context(), "s1", "sea1") })
	checkImage("GetEpisodeThumbnail", func() (io.ReadCloser, string, error) { return p.GetEpisodeThumbnail(t.Context(), "s1", "sea1", "ep1") })
}

func TestFileEndpoints(t *testing.T) {
	streamData := []byte("fake-stream-data")
	subData := []byte("fake-sub-data")
	p := stub.New(stub.Config{
		StreamFileData:   streamData,
		StreamFileMIME:   "video/mp4",
		SubtitleFileData: subData,
		SubtitleFileMIME: "text/vtt",
	})

	checkFile := func(name string, fn func() (io.ReadCloser, string, error)) {
		t.Helper()
		rc, mime, err := fn()
		if err != nil {
			t.Fatalf("%s() error: %v", name, err)
		}
		if mime != "video/mp4" && mime != "text/vtt" {
			t.Errorf("%s() mime = %q, want %q or %q", name, mime, "video/mp4", "text/vtt")
		}
		got, _ := io.ReadAll(rc)
		rc.Close()
		if len(got) == 0 {
			t.Errorf("%s() returned empty data", name)
		}
	}

	checkFile("GetMovieStreamFile", func() (io.ReadCloser, string, error) { return p.GetMovieStreamFile(t.Context(), "m1", "manifest.mpd") })
	checkFile("GetMovieSubtitleFile", func() (io.ReadCloser, string, error) { return p.GetMovieSubtitleFile(t.Context(), "m1", "en.vtt") })
	checkFile("GetEpisodeStreamFile", func() (io.ReadCloser, string, error) { return p.GetEpisodeStreamFile(t.Context(), "s1", "sea1", "ep1", "manifest.mpd") })
	checkFile("GetEpisodeSubtitleFile", func() (io.ReadCloser, string, error) { return p.GetEpisodeSubtitleFile(t.Context(), "s1", "sea1", "ep1", "en.vtt") })
}

func TestSearch(t *testing.T) {
	results := []oas.SearchResultItem{
		{Score: 1.0, Resource: oas.NewMovieSearchResultItemResource(oas.Movie{ID: "m1", Name: "Found"})},
	}
	p := stub.New(stub.Config{Search: results})

	t.Run("configured", func(t *testing.T) {
		got, total, err := p.Search(t.Context(), "test", 10, 0)
		if err != nil {
			t.Fatalf("Search() error: %v", err)
		}
		if total != 1 {
			t.Errorf("total = %d, want 1", total)
		}
		if len(got) != 1 {
			t.Errorf("got %d items, want 1", len(got))
		}
	})

	t.Run("not configured", func(t *testing.T) {
		p := stub.New(stub.Config{Tag: "test"})
		_, _, err := p.Search(t.Context(), "test", 10, 0)
		if err != provider.ErrNotSupported {
			t.Errorf("Search() error = %v, want ErrNotSupported", err)
		}
	})
}

func TestNilConfig(t *testing.T) {
	p := stub.New(stub.Config{Tag: "empty"})

	checkNotSupported := func(name string, err error) {
		t.Helper()
		if err != provider.ErrNotSupported {
			t.Errorf("%s error = %v, want ErrNotSupported", name, err)
		}
	}

	checkNotSupported("Service", func() error { _, err := p.Service(t.Context()); return err }())
	checkNotSupported("Health", func() error { _, err := p.Health(t.Context()); return err }())
	checkNotSupported("GetMovies", func() error { _, _, err := p.GetMovies(t.Context(), 10, 0); return err }())
	checkNotSupported("GetMovieById", func() error { _, err := p.GetMovieById(t.Context(), "x"); return err }())
	checkNotSupported("GetSeries", func() error { _, _, err := p.GetSeries(t.Context(), 10, 0); return err }())
	checkNotSupported("GetSeriesByID", func() error { _, err := p.GetSeriesByID(t.Context(), "x"); return err }())
	checkNotSupported("GetSeasons", func() error { _, _, err := p.GetSeasons(t.Context(), "x", 10, 0); return err }())
	checkNotSupported("GetSeasonById", func() error { _, err := p.GetSeasonById(t.Context(), "x", "y"); return err }())
	checkNotSupported("GetEpisodes", func() error { _, _, err := p.GetEpisodes(t.Context(), "x", "y", 10, 0); return err }())
	checkNotSupported("GetEpisodeById", func() error { _, err := p.GetEpisodeById(t.Context(), "x", "y", "z"); return err }())
	checkNotSupported("GetMoviePoster", func() error { _, _, err := p.GetMoviePoster(t.Context(), "x"); return err }())
	checkNotSupported("GetMovieStreamFile", func() error { _, _, err := p.GetMovieStreamFile(t.Context(), "x", "y"); return err }())
	checkNotSupported("GetEpisodeThumbnail", func() error { _, _, err := p.GetEpisodeThumbnail(t.Context(), "x", "y", "z"); return err }())
	checkNotSupported("Search", func() error { _, _, err := p.Search(t.Context(), "q", 10, 0); return err }())
}
