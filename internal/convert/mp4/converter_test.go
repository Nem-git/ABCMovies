package mp4

import (
	"bytes"
	"context"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"

	"github.com/nem-git/abcmovies/internal/drm"
	"github.com/nem-git/abcmovies/internal/stream"
)

// fakeFetcher serves fixture content from a URL map.
type fakeFetcher struct {
	files map[string]string
}

func (f *fakeFetcher) Fetch(_ context.Context, rawURL string, _ http.Header, _ url.Values) (io.ReadCloser, http.Header, error) {
	body, ok := f.files[rawURL]
	if !ok {
		return nil, nil, io.ErrUnexpectedEOF
	}
	return io.NopCloser(strings.NewReader(body)), http.Header{"Content-Type": []string{"video/mp4"}}, nil
}

// buildFMP4 builds a minimal single-track fMP4 init + n media segments.
func buildFMP4(t *testing.T, trackID uint32, ts uint32, n int) (init []byte, segs [][]byte) {
	t.Helper()
	initSeg := mp4.CreateEmptyInit()
	trak := initSeg.AddEmptyTrack(ts, "video", "und")
	sps, err := hex.DecodeString("67640020accac05005bb0169e0000003002000000c9c4c000432380008647c12401cb1c31380")
	if err != nil {
		t.Fatalf("decoding sps: %v", err)
	}
	pps, err := hex.DecodeString("68ce3c80")
	if err != nil {
		t.Fatalf("decoding pps: %v", err)
	}
	if err := trak.SetAVCDescriptor("avc1", [][]byte{sps}, [][]byte{pps}, true); err != nil {
		t.Fatalf("SetAVCDescriptor: %v", err)
	}
	var buf bytes.Buffer
	if err := initSeg.Encode(&buf); err != nil {
		t.Fatalf("encoding init: %v", err)
	}
	init = buf.Bytes()

	for i := 0; i < n; i++ {
		frag, err := mp4.CreateFragment(uint32(i+1), trackID)
		if err != nil {
			t.Fatalf("CreateFragment: %v", err)
		}
		fs := mp4.FullSample{
			Sample: mp4.Sample{
				Flags: mp4.SyncSampleFlags,
				Dur:   3000,
				Size:  4,
			},
			DecodeTime: uint64(i * 3000),
			Data:       []byte{0x00, 0x00, 0x00, 0x01},
		}
		if err := frag.AddFullSampleToTrack(fs, trackID); err != nil {
			t.Fatalf("AddFullSampleToTrack: %v", err)
		}
		seg := mp4.NewMediaSegmentWithoutStyp()
		seg.AddFragment(frag)
		var sbuf bytes.Buffer
		if err := seg.Encode(&sbuf); err != nil {
			t.Fatalf("encoding segment %d: %v", i, err)
		}
		segs = append(segs, sbuf.Bytes())
	}
	return init, segs
}

func decodeOutput(t *testing.T, out []byte) *mp4.File {
	t.Helper()
	f, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(out))
	if err != nil {
		t.Fatalf("decoding output mp4: %v", err)
	}
	return f
}

// fragmentCount sums the fragments across all media segments.
func fragmentCount(f *mp4.File) int {
	n := 0
	for _, seg := range f.Segments {
		n += len(seg.Fragments)
	}
	return n
}

func TestSupports(t *testing.T) {
	c := NewConverter(nil)
	if !c.Supports("dash", "mp4") || !c.Supports("hls", "mp4") {
		t.Error("expected dash->mp4 and hls->mp4 support")
	}
	if c.Supports("mp4", "hls") || c.Supports("dash", "hls") {
		t.Error("unexpected conversion support")
	}
}

func TestConvertDASHSingleTrack(t *testing.T) {
	init, segs := buildFMP4(t, 1, 90000, 2)
	mpd := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" mediaPresentationDuration="PT6S" minBufferTime="PT1S" type="static">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video" segmentAlignment="true">
      <Representation id="v0" bandwidth="1000000" codecs="avc1.42e01e" width="1280" height="720">
        <SegmentTemplate media="seg-$Number$.m4s" initialization="init.mp4" timescale="90000" duration="270000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`
	f := &fakeFetcher{files: map[string]string{
		"http://upstream/manifest.mpd": mpd,
		"http://upstream/init.mp4":     string(init),
		"http://upstream/seg-1.m4s":    string(segs[0]),
		"http://upstream/seg-2.m4s":    string(segs[1]),
	}}

	c := NewConverter(f)
	var out bytes.Buffer
	err := c.Convert(context.Background(), &stream.Locator{
		URL:            "http://upstream/manifest.mpd",
		EncodingFormat: "application/dash+xml",
	}, &out)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	file := decodeOutput(t, out.Bytes())
	if file.Init == nil || file.Init.Moov == nil || len(file.Init.Moov.Traks) != 1 {
		t.Fatalf("expected 1 track in output, got %d", len(file.Init.Moov.Traks))
	}
	if fragmentCount(file) != 2 {
		t.Errorf("expected 2 fragments, got %d", fragmentCount(file))
	}
}

func TestConvertDASHMultiTrack(t *testing.T) {
	vInit, vSegs := buildFMP4(t, 1, 90000, 2)
	mpd := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" mediaPresentationDuration="PT6S" minBufferTime="PT1S" type="static">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <Representation id="v0" bandwidth="2000000">
        <SegmentTemplate media="v-$Number$.m4s" initialization="v-init.mp4" timescale="90000" duration="270000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
    <AdaptationSet mimeType="audio/mp4" contentType="audio">
      <Representation id="a0" bandwidth="128000">
        <SegmentTemplate media="a-$Number$.m4s" initialization="a-init.mp4" timescale="48000" duration="144000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`
	files := map[string]string{
		"http://upstream/manifest.mpd": mpd,
		"http://upstream/v-init.mp4":   string(vInit),
		"http://upstream/v-1.m4s":      string(vSegs[0]),
		"http://upstream/v-2.m4s":      string(vSegs[1]),
	}
	// The audio fixture is structure-only; any single-track fMP4 works.
	aInit, aSegs := buildFMP4(t, 1, 48000, 2)
	files["http://upstream/a-init.mp4"] = string(aInit)
	files["http://upstream/a-1.m4s"] = string(aSegs[0])
	files["http://upstream/a-2.m4s"] = string(aSegs[1])

	f := &fakeFetcher{files: files}
	c := NewConverter(f)
	var out bytes.Buffer
	err := c.Convert(context.Background(), &stream.Locator{
		URL:            "http://upstream/manifest.mpd",
		EncodingFormat: "application/dash+xml",
	}, &out)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	file := decodeOutput(t, out.Bytes())
	if len(file.Init.Moov.Traks) != 2 {
		t.Fatalf("expected 2 tracks in merged output, got %d", len(file.Init.Moov.Traks))
	}
	if fragmentCount(file) != 2 {
		t.Errorf("expected 2 fragments in merged output, got %d", fragmentCount(file))
	}
}

func TestConvertHLSFMP4(t *testing.T) {
	init, segs := buildFMP4(t, 1, 90000, 2)
	media := `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:3
#EXT-X-MAP:URI="init.mp4"
#EXTINF:3.0,
seg-1.m4s
#EXTINF:3.0,
seg-2.m4s
#EXT-X-ENDLIST
`
	master := `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-STREAM-INF:BANDWIDTH=1000000,RESOLUTION=1280x720
media.m3u8
#EXT-X-ENDLIST
`
	f := &fakeFetcher{files: map[string]string{
		"http://upstream/master.m3u8": master,
		"http://upstream/media.m3u8":  media,
		"http://upstream/init.mp4":    string(init),
		"http://upstream/seg-1.m4s":   string(segs[0]),
		"http://upstream/seg-2.m4s":   string(segs[1]),
	}}

	c := NewConverter(f)
	var out bytes.Buffer
	err := c.Convert(context.Background(), &stream.Locator{
		URL:            "http://upstream/master.m3u8",
		EncodingFormat: "application/vnd.apple.mpegurl",
	}, &out)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	file := decodeOutput(t, out.Bytes())
	if len(file.Init.Moov.Traks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(file.Init.Moov.Traks))
	}
	if fragmentCount(file) != 2 {
		t.Errorf("expected 2 fragments, got %d", fragmentCount(file))
	}
}

func TestConvertHLSMediaPlaylistDirect(t *testing.T) {
	init, segs := buildFMP4(t, 1, 90000, 1)
	media := `#EXTM3U
#EXT-X-VERSION:6
#EXT-X-TARGETDURATION:3
#EXT-X-MAP:URI="init.mp4"
#EXTINF:3.0,
seg-1.m4s
#EXT-X-ENDLIST
`
	f := &fakeFetcher{files: map[string]string{
		"http://upstream/media.m3u8": media,
		"http://upstream/init.mp4":   string(init),
		"http://upstream/seg-1.m4s":  string(segs[0]),
	}}
	c := NewConverter(f)
	var out bytes.Buffer
	err := c.Convert(context.Background(), &stream.Locator{
		URL:            "http://upstream/media.m3u8",
		EncodingFormat: "application/vnd.apple.mpegurl",
	}, &out)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	file := decodeOutput(t, out.Bytes())
	if fragmentCount(file) != 1 {
		t.Errorf("expected 1 fragment, got %d", fragmentCount(file))
	}
}

func TestConvertTSInvalid(t *testing.T) {
	// A playlist of .ts segments whose payload is not MPEG-TS should fail
	// cleanly rather than panic or hang.
	media := `#EXTM3U
#EXT-X-VERSION:3
#EXT-X-TARGETDURATION:3
#EXTINF:3.0,
seg-1.ts
#EXT-X-ENDLIST
`
	f := &fakeFetcher{files: map[string]string{
		"http://upstream/media.m3u8": media,
		"http://upstream/seg-1.ts":   "this is not mpeg-ts data",
	}}
	c := NewConverter(f)
	var out bytes.Buffer
	err := c.Convert(context.Background(), &stream.Locator{
		URL:            "http://upstream/media.m3u8",
		EncodingFormat: "application/vnd.apple.mpegurl",
	}, &out)
	if err == nil {
		t.Fatal("expected error for non-TS payload")
	}
	if out.Len() != 0 {
		t.Errorf("expected no partial output, got %d bytes", out.Len())
	}
}

func TestConvertErrors(t *testing.T) {
	c := NewConverter(&fakeFetcher{files: map[string]string{}})

	t.Run("nil locator", func(t *testing.T) {
		if err := c.Convert(context.Background(), nil, io.Discard); err == nil {
			t.Error("expected error for nil locator")
		}
	})

	t.Run("unsupported encoding", func(t *testing.T) {
		err := c.Convert(context.Background(), &stream.Locator{URL: "x", EncodingFormat: "text/plain"}, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "unsupported source encoding") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("empty DASH", func(t *testing.T) {
		f := &fakeFetcher{files: map[string]string{"http://u/manifest.mpd": ""}}
		err := NewConverter(f).Convert(context.Background(), &stream.Locator{URL: "http://u/manifest.mpd", EncodingFormat: "application/dash+xml"}, io.Discard)
		if err == nil {
			t.Error("expected error for empty MPD")
		}
	})
}

// TestConvertDASHSingleTrackDRM verifies the DRM-enabled converter decrypts
// CENC-encrypted init + media segments using a ClearKey provider before
// concatenating the output MP4.
func TestConvertDASHSingleTrackDRM(t *testing.T) {
	clearInit, clearSegs := buildFMP4(t, 1, 90000, 2)

	key, err := hex.DecodeString("00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	iv, err := hex.DecodeString("7766554433221100")
	if err != nil {
		t.Fatal(err)
	}
	kidUUID, err := mp4.NewUUIDFromString("11112222333344445555666677778888")
	if err != nil {
		t.Fatal(err)
	}

	// Build a ClearKey PSSH so the engine can auto-detect the scheme.
	pssh, err := mp4.NewPsshBox("e2719d58a985b3c9781ab030af78d8e5", []string{"11112222333344445555666677778888"}, []byte("clearkey-pssh"))
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt init.
	init, err := mp4.DecodeFile(bytes.NewBuffer(clearInit))
	if err != nil {
		t.Fatal(err)
	}
	ipf, err := mp4.InitProtect(init.Init, key, iv, "cenc", kidUUID, []*mp4.PsshBox{pssh})
	if err != nil {
		t.Fatal(err)
	}
	var encInitBuf bytes.Buffer
	if err := init.Encode(&encInitBuf); err != nil {
		t.Fatal(err)
	}

	// Encrypt segments.
	encSegs := make([][]byte, len(clearSegs))
	for i, clearSeg := range clearSegs {
		seg, err := mp4.DecodeFile(bytes.NewBuffer(clearSeg))
		if err != nil {
			t.Fatal(err)
		}
		fragIV := iv
		for _, s := range seg.Segments {
			for _, f := range s.Fragments {
				fragIV, err = mp4.EncryptFragment(f, key, fragIV, ipf)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
		var b bytes.Buffer
		if err := seg.Encode(&b); err != nil {
			t.Fatal(err)
		}
		encSegs[i] = b.Bytes()
	}

	mpd := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" mediaPresentationDuration="PT6S" minBufferTime="PT1S" type="static">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <Representation id="v0" bandwidth="1000000">
        <SegmentTemplate media="seg-$Number$.m4s" initialization="init.mp4" timescale="90000" duration="270000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`
	files := map[string]string{
		"http://upstream/manifest.mpd": mpd,
		"http://upstream/init.mp4":     string(encInitBuf.Bytes()),
		"http://upstream/seg-1.m4s":    string(encSegs[0]),
		"http://upstream/seg-2.m4s":    string(encSegs[1]),
	}
	f := &fakeFetcher{files: files}

	engine, err := drm.BuildEngine(drm.Config{
		Enabled: true,
		ClearKey: drm.ClearKeyConfig{
			Keys: map[string]string{"11112222333344445555666677778888": "00112233445566778899aabbccddeeff"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	c := NewConverterWithDRM(f, engine)

	var out bytes.Buffer
	err = c.Convert(context.Background(), &stream.Locator{
		URL:            "http://upstream/manifest.mpd",
		EncodingFormat: "application/dash+xml",
		ProviderTag:    "TEST",
		ContentKey:     "movie:1",
	}, &out)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	// Output must have exactly one track and two fragments, with no leftover
	// encryption (no pssh/sinf/senc) and decrypted sample payloads.
	file := decodeOutput(t, out.Bytes())
	if file.Init == nil || file.Init.Moov == nil || len(file.Init.Moov.Traks) != 1 {
		t.Fatalf("expected 1 track in output, got %d", len(file.Init.Moov.Traks))
	}
	if len(file.Init.Moov.Psshs) != 0 {
		t.Errorf("cleaned init still has %d pssh boxes", len(file.Init.Moov.Psshs))
	}
	if fragmentCount(file) != 2 {
		t.Errorf("expected 2 fragments, got %d", fragmentCount(file))
	}
	// Verify first segment sample payload matches the clear payload.
	for _, seg := range file.Segments {
		for _, frag := range seg.Fragments {
			fss, err := frag.GetFullSamples(file.Init.Moov.Mvex.Trex)
			if err != nil {
				t.Fatalf("GetFullSamples: %v", err)
			}
			for _, fs := range fss {
				if !bytes.Equal(fs.Data, []byte{0x00, 0x00, 0x00, 0x01}) {
					t.Errorf("sample data not decrypted: %x", fs.Data)
				}
			}
		}
	}
}

// TestConvertDASHDRMDisabled verifies the DRM-enabled converter passes clear
// content through unchanged when the engine has no providers (drm disabled).
func TestConvertDASHDRMDisabled(t *testing.T) {
	init, segs := buildFMP4(t, 1, 90000, 1)
	mpd := `<?xml version="1.0" encoding="UTF-8"?>
<MPD xmlns="urn:mpeg:dash:schema:mpd:2011" mediaPresentationDuration="PT3S" minBufferTime="PT1S" type="static">
  <Period>
    <AdaptationSet mimeType="video/mp4" contentType="video">
      <Representation id="v0" bandwidth="1000000">
        <SegmentTemplate media="seg-$Number$.m4s" initialization="init.mp4" timescale="90000" duration="270000" startNumber="1"/>
      </Representation>
    </AdaptationSet>
  </Period>
</MPD>`
	f := &fakeFetcher{files: map[string]string{
		"http://upstream/manifest.mpd": mpd,
		"http://upstream/init.mp4":     string(init),
		"http://upstream/seg-1.m4s":    string(segs[0]),
	}}
	engine, err := drm.BuildEngine(drm.Config{Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	c := NewConverterWithDRM(f, engine)
	var out bytes.Buffer
	if err := c.Convert(context.Background(), &stream.Locator{
		URL:            "http://upstream/manifest.mpd",
		EncodingFormat: "application/dash+xml",
	}, &out); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	file := decodeOutput(t, out.Bytes())
	if fragmentCount(file) != 1 {
		t.Errorf("expected 1 fragment, got %d", fragmentCount(file))
	}
}
