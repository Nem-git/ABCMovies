package handler_test

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nem-git/abcmovies/internal/convert"
	"github.com/nem-git/abcmovies/internal/handler"
	"github.com/nem-git/abcmovies/internal/oas"
	"github.com/nem-git/abcmovies/internal/providers/stub"
	"github.com/nem-git/abcmovies/internal/proxy"
	"github.com/nem-git/abcmovies/internal/registry"
	"github.com/nem-git/abcmovies/internal/stream"
)

// fetcher serves fixture content from a URL map.
type fetcher struct {
	files map[string]string
}

func (f *fetcher) Fetch(_ context.Context, rawURL string, _ http.Header, _ url.Values) (io.ReadCloser, http.Header, error) {
	body, ok := f.files[rawURL]
	if !ok {
		return nil, nil, io.ErrUnexpectedEOF
	}
	return io.NopCloser(strings.NewReader(body)), http.Header{"Content-Type": []string{"video/mp4"}}, nil
}

const dashFixtureMPD = `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" mediaPresentationDuration="PT6S" minBufferTime="PT1S" type="static">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video" segmentAlignment="true">
      <Representation id="v0" bandwidth="1000000" codecs="avc1.42e01e" width="1280" height="720">
        <SegmentTemplate media="seg-$Number$.m4s" initialization="init.mp4" timescale="90000" duration="270000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`

func TestGetMovieStreamFileConverted(t *testing.T) {
	init, seg1, seg2 := "init-bytes", "segment-one", "segment-two"
	fetcher := &fetcher{files: map[string]string{
		"http://upstream/manifest.mpd": dashFixtureMPD,
		"http://upstream/init.mp4":     init,
		"http://upstream/seg-1.m4s":    seg1,
		"http://upstream/seg-2.m4s":    seg2,
	}}

	r := registry.New()
	r.Register(stub.New(stub.Config{
		Tag: "conv",
		StreamLocators: map[string]*stream.Locator{
			"manifest.mpd": {URL: "http://upstream/manifest.mpd", EncodingFormat: "application/dash+xml"},
		},
	}))

	px := proxy.New(proxy.Dependencies{
		Fetcher: fetcher,
		State:   proxy.NewMemoryStore(5 * time.Minute),
		Configs: map[string]*proxy.Config{"conv": {Strategy: "auto", Convert: true}},
	})
	h := handler.New(r, "", "", px).WithConverters(convert.NewRegistry(fetcher))

	res, err := h.GetMovieStreamFile(t.Context(), oas.GetMovieStreamFileParams{
		ServiceTag: "conv",
		MovieId:    "m1",
		Format:     oas.StreamFormatMP4,
	})
	if err != nil {
		t.Fatalf("GetMovieStreamFile() error: %v", err)
	}
	mp4, ok := res.(*oas.GetMovieStreamFileOKVideoMP4)
	if !ok {
		t.Fatalf("response = %T, want *oas.GetMovieStreamFileOKVideoMP4", res)
	}
	got, err := io.ReadAll(mp4.Data)
	if err != nil {
		t.Fatalf("reading converted output: %v", err)
	}
	want := init + seg1 + seg2
	if string(got) != want {
		t.Errorf("converted output = %q, want %q", got, want)
	}
}

func TestGetEpisodeStreamFileConverted(t *testing.T) {
	fetcher := &fetcher{files: map[string]string{
		"http://upstream/manifest.mpd": dashFixtureMPD,
		"http://upstream/init.mp4":     "init",
		"http://upstream/seg-1.m4s":    "one",
		"http://upstream/seg-2.m4s":    "two",
	}}

	r := registry.New()
	r.Register(stub.New(stub.Config{
		Tag: "conv",
		StreamLocators: map[string]*stream.Locator{
			"manifest.mpd": {URL: "http://upstream/manifest.mpd", EncodingFormat: "application/dash+xml"},
		},
	}))
	px := proxy.New(proxy.Dependencies{
		Fetcher: fetcher,
		State:   proxy.NewMemoryStore(5 * time.Minute),
		Configs: map[string]*proxy.Config{"conv": {Strategy: "auto", Convert: true}},
	})
	h := handler.New(r, "", "", px).WithConverters(convert.NewRegistry(fetcher))

	res, err := h.GetEpisodeStreamFile(t.Context(), oas.GetEpisodeStreamFileParams{
		ServiceTag: "conv",
		SeriesId:   "s1",
		SeasonId:   "sea1",
		EpisodeId:  "ep1",
		Format:     oas.StreamFormatMP4,
	})
	if err != nil {
		t.Fatalf("GetEpisodeStreamFile() error: %v", err)
	}
	mp4, ok := res.(*oas.GetEpisodeStreamFileOKVideoMP4)
	if !ok {
		t.Fatalf("response = %T, want *oas.GetEpisodeStreamFileOKVideoMP4", res)
	}
	got, err := io.ReadAll(mp4.Data)
	if err != nil {
		t.Fatalf("reading converted output: %v", err)
	}
	if string(got) != "initonetwo" {
		t.Errorf("converted output = %q, want %q", got, "initonetwo")
	}
}

func TestGetMovieStreamFileConvertDisabled(t *testing.T) {
	r := registry.New()
	r.Register(stub.New(stub.Config{
		Tag: "conv",
		StreamLocators: map[string]*stream.Locator{
			"manifest.mpd": {URL: "http://upstream/manifest.mpd", EncodingFormat: "application/dash+xml"},
		},
	}))
	px := proxy.New(proxy.Dependencies{
		Fetcher: &fetcher{},
		State:   proxy.NewMemoryStore(5 * time.Minute),
		Configs: map[string]*proxy.Config{"conv": {Strategy: "auto"}},
	})
	h := handler.New(r, "", "", px).WithConverters(convert.NewRegistry(&fetcher{}))

	res, err := h.GetMovieStreamFile(t.Context(), oas.GetMovieStreamFileParams{
		ServiceTag: "conv",
		MovieId:    "m1",
		Format:     oas.StreamFormatMP4,
	})
	if err != nil {
		t.Fatalf("expected 404 response, got error: %v", err)
	}
	nf, ok := res.(*oas.GetMovieStreamFileNotFound)
	if !ok {
		t.Fatalf("response = %T, want *oas.GetMovieStreamFileNotFound", res)
	}
	if nf.Code != "STREAM_NOT_FOUND" {
		t.Errorf("Code = %q, want %q", nf.Code, "STREAM_NOT_FOUND")
	}
}

func TestGetMovieStreamFileNotFound(t *testing.T) {
	r := registry.New()
	r.Register(stub.New(stub.Config{Tag: "conv"}))
	h := handler.New(r, "", "").WithConverters(convert.NewRegistry(&fetcher{}))

	res, err := h.GetMovieStreamFile(t.Context(), oas.GetMovieStreamFileParams{
		ServiceTag: "conv",
		MovieId:    "m1",
		Format:     oas.StreamFormatMP4,
	})
	if err != nil {
		t.Fatalf("expected 404 response, got error: %v", err)
	}
	nf, ok := res.(*oas.GetMovieStreamFileNotFound)
	if !ok {
		t.Fatalf("response = %T, want *oas.GetMovieStreamFileNotFound", res)
	}
	if nf.Code != "STREAM_NOT_FOUND" {
		t.Errorf("Code = %q, want %q", nf.Code, "STREAM_NOT_FOUND")
	}
}

func TestGetEpisodeStreamFileNotFound(t *testing.T) {
	r := registry.New()
	r.Register(stub.New(stub.Config{Tag: "conv"}))
	h := handler.New(r, "", "").WithConverters(convert.NewRegistry(&fetcher{}))

	res, err := h.GetEpisodeStreamFile(t.Context(), oas.GetEpisodeStreamFileParams{
		ServiceTag: "conv",
		SeriesId:   "s1",
		SeasonId:   "sea1",
		EpisodeId:  "ep1",
		Format:     oas.StreamFormatMP4,
	})
	if err != nil {
		t.Fatalf("expected 404 response, got error: %v", err)
	}
	nf, ok := res.(*oas.GetEpisodeStreamFileNotFound)
	if !ok {
		t.Fatalf("response = %T, want *oas.GetEpisodeStreamFileNotFound", res)
	}
	if nf.Code != "STREAM_NOT_FOUND" {
		t.Errorf("Code = %q, want %q", nf.Code, "STREAM_NOT_FOUND")
	}
}

func TestGetMovieStreamFileDirectMp4(t *testing.T) {
	r := registry.New()
	r.Register(stub.New(stub.Config{
		Tag:            "native",
		StreamFileData: []byte("native-mp4"),
		StreamFileMIME: "video/mp4",
	}))
	h := handler.New(r, "", "")

	res, err := h.GetMovieStreamFile(t.Context(), oas.GetMovieStreamFileParams{
		ServiceTag: "native",
		MovieId:    "m1",
		Format:     oas.StreamFormatMP4,
	})
	if err != nil {
		t.Fatalf("GetMovieStreamFile() error: %v", err)
	}
	mp4, ok := res.(*oas.GetMovieStreamFileOKVideoMP4)
	if !ok {
		t.Fatalf("response = %T, want *oas.GetMovieStreamFileOKVideoMP4", res)
	}
	got, err := io.ReadAll(mp4.Data)
	if err != nil {
		t.Fatalf("reading response: %v", err)
	}
	if string(got) != "native-mp4" {
		t.Errorf("body = %q, want %q", got, "native-mp4")
	}
}
