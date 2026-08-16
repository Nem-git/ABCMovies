package drm

import (
	"bytes"
	"context"
	"encoding/hex"
	"testing"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
)

// TestEngineRoundTrip encrypts a known-good fMP4 with mp4ff (cenc), then
// decrypts it through the Engine and verifies the output matches the clear data.
func TestEngineRoundTrip(t *testing.T) {
	keyHex := "00112233445566778899aabbccddeeff"
	ivHex := "7766554433221100"
	kidHex := "11112222333344445555666677778888"

	key, _ := hex.DecodeString(keyHex)
	kidUUID, _ := mp4.NewUUIDFromString(kidHex)

	rawInit, err := readTestData("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	rawSeg, err := readTestData("testdata/1.m4s")
	if err != nil {
		t.Fatal(err)
	}

	// Encrypt the init segment. Decode via an io.Reader so the parser copies
	// the bytes: DecodeFileSR views the caller's slice, and EncryptFragment
	// would mutate rawInit/rawSeg in place, corrupting the expected output.
	init, err := mp4.DecodeFile(bytes.NewBuffer(rawInit))
	if err != nil {
		t.Fatal(err)
	}
	ipf, err := mp4.InitProtect(init.Init, key, ivMustDecode(ivHex), "cenc", kidUUID, nil)
	if err != nil {
		t.Fatal(err)
	}
	var encInitBuf bytes.Buffer
	if err := init.Encode(&encInitBuf); err != nil {
		t.Fatal(err)
	}

	// Encrypt the media segment.
	seg, err := mp4.DecodeFile(bytes.NewBuffer(rawSeg))
	if err != nil {
		t.Fatal(err)
	}
	fragIV := ivMustDecode(ivHex)
	for _, s := range seg.Segments {
		for _, f := range s.Fragments {
			fragIV, err = mp4.EncryptFragment(f, key, fragIV, ipf)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	var encSegBuf bytes.Buffer
	if err := seg.Encode(&encSegBuf); err != nil {
		t.Fatal(err)
	}

	// Build the engine with a static key provider.
	kid := kidFromUUID(kidUUID)
	provider := &fakeProvider{
		scheme: SchemeWidevine,
		keys:   map[KID]CEK{kid: CEK(key)},
	}
	engine := NewEngine(provider)

	encInit, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(encInitBuf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := engine.PrepareInit(context.Background(), encInit.Init, KeyRequest{
		ProviderTag: "T",
		ContentKey:  "movie:1",
		Scheme:      SchemeWidevine,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Cleaned init should have no PSSH and no sinf/tenc.
	if len(stream.CleanInit.Moov.Psshs) != 0 {
		t.Errorf("cleaned init still has %d pssh boxes", len(stream.CleanInit.Moov.Psshs))
	}

	decrypted, err := stream.DecryptSegment(encSegBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	// Decrypted segment should match the original clear segment.
	if !bytes.Equal(decrypted, rawSeg) {
		t.Errorf("decrypted segment differs from original")
	}
}

// TestEngineClearPassthrough verifies an unprotected init passes through unchanged.
func TestEngineClearPassthrough(t *testing.T) {
	rawInit, err := readTestData("testdata/init.mp4")
	if err != nil {
		t.Fatal(err)
	}
	engine := NewEngine(nil)
	init, err := mp4.DecodeFileSR(bits.NewFixedSliceReader(rawInit))
	if err != nil {
		t.Fatal(err)
	}
	stream, err := engine.PrepareInit(context.Background(), init.Init, KeyRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if stream.CleanInit != init.Init {
		t.Errorf("expected passthrough for clear init")
	}
}

// fakeProvider is a static KeyProvider for tests.
type fakeProvider struct {
	scheme Scheme
	keys   map[KID]CEK
}

func (p *fakeProvider) Scheme() Scheme { return p.scheme }

func (p *fakeProvider) GetKeys(_ context.Context, req KeyRequest) (map[KID]CEK, error) {
	out := make(map[KID]CEK)
	for _, kid := range req.KIDs {
		if k, ok := p.keys[kid]; ok {
			out[kid] = k
		}
	}
	if len(out) == 0 {
		return nil, ErrCEKNotFound
	}
	return out, nil
}

func readTestData(path string) ([]byte, error) {
	// helper to avoid importing os in every test
	return readFile(path)
}
