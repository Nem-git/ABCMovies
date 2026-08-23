package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
	slotsv1 "github.com/nem-git/abcmovies/core/gen/abcmovies/slots/v1"
)

// The load-bearing contracts (PLAN.md §2.3), exercised identically here and in
// the fixture runner. The round-trip tests prove the wire format is lossless
// (TESTING.md §4.1), and the unknown-field test proves an older consumer does
// not destroy a newer message (PLAN.md §3.4 additive versioning).
var loadBearingContracts = []string{
	"library_entry",
	"media_source",
	"job",
	"event",
	"title_metadata",
	// The provider slot's whole-catalogue sync contract — the messages the
	// account source cache persists. Its suite keeps both sync directions in
	// one directory, distinguished by each case's type field.
	"provider_sync_request",
	"provider_sync_response",
}

func newMessage(contract string) proto.Message {
	switch contract {
	case "library_entry":
		return &corev1.LibraryEntry{}
	case "media_source":
		return &corev1.MediaSource{}
	case "job":
		return &corev1.Job{}
	case "event":
		return &corev1.EventEnvelope{}
	case "title_metadata":
		return &corev1.TitleMetadata{}
	case "provider_sync_request":
		return &slotsv1.CatalogueSyncRequest{}
	case "provider_sync_response":
		return &slotsv1.CatalogueSyncResponse{}
	}
	panic("unknown contract " + contract)
}

// fixtureCase is the subset of a positive fixture case the round-trip tests
// need; the authoritative shape lives in fixtures/<contract>/v1/positive/.
type fixtureCase struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Message json.RawMessage `json:"message"`
}

// testB is the subset of *testing.T / *testing.F the fixture helpers need.
type testB interface {
	Helper()
	Fatalf(string, ...any)
}

func loadPositiveFixtures(t testB, contract string) []fixtureCase {
	t.Helper()
	// Contracts sharing a fixture directory with sibling message types read
	// only the cases whose type matches (see loadBearingContracts).
	dir, wantType := contract, ""
	switch contract {
	case "provider_sync_request":
		dir, wantType = "provider", "CatalogueSyncRequest"
	case "provider_sync_response":
		dir, wantType = "provider", "CatalogueSyncResponse"
	}
	root := filepath.Join("..", "..", "..", "fixtures", dir, "v1", "positive", "cases")
	paths, err := filepath.Glob(filepath.Join(root, "*.json"))
	if err != nil {
		t.Fatalf("%s: glob positive cases: %v", contract, err)
	}
	cases := make([]fixtureCase, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("%s: read %s: %v", contract, p, err)
		}
		var c fixtureCase
		if err := json.Unmarshal(b, &c); err != nil {
			t.Fatalf("%s: parse %s: %v", contract, p, err)
		}
		if wantType != "" && c.Type != wantType {
			continue
		}
		cases = append(cases, c)
	}
	if len(cases) == 0 {
		t.Fatalf("%s: no positive fixture cases found under %s", contract, root)
	}
	return cases
}

func parsePositiveFixture(t testB, contract string, c fixtureCase) proto.Message {
	t.Helper()
	m := newMessage(contract)
	if err := protojson.Unmarshal(c.Message, m); err != nil {
		t.Fatalf("%s: case %q: %v", contract, c.Name, err)
	}
	return m
}

// hasUnknownField scans raw protobuf wire bytes for a varint field with the
// given field number and expected value. Returns true if found.
func hasUnknownField(b []byte, fieldNum uint64, expectedVal uint64) bool {
	for len(b) > 0 {
		num, wtype, tagLen := protowire.ConsumeTag(b)
		if tagLen < 0 {
			return false
		}
		b = b[tagLen:]
		if num == protowire.Number(fieldNum) && wtype == protowire.VarintType {
			val, valLen := protowire.ConsumeVarint(b)
			if valLen < 0 {
				return false
			}
			return val == expectedVal
		}
		valLen := protowire.ConsumeFieldValue(num, wtype, b)
		if valLen < 0 {
			return false
		}
		b = b[valLen:]
	}
	return false
}

// TestRoundTripLossless proves encode(parse(encode(x))) == encode(x) in both
// binary and JSON, over every positive fixture of every load-bearing contract
// (TESTING.md §4.1).
func TestRoundTripLossless(t *testing.T) {
	for _, contract := range loadBearingContracts {
		for _, c := range loadPositiveFixtures(t, contract) {
			m := parsePositiveFixture(t, contract, c)
			first, err := proto.Marshal(m)
			if err != nil {
				t.Fatalf("%s/%s: marshal: %v", contract, c.Name, err)
			}

			parsed := newMessage(contract)
			if err := proto.Unmarshal(first, parsed); err != nil {
				t.Fatalf("%s/%s: unmarshal: %v", contract, c.Name, err)
			}
			// Compare semantically: protobuf maps do not guarantee a
			// deterministic wire order, so byte-for-byte equality does not
			// hold for messages that contain maps.
			if !proto.Equal(m, parsed) {
				t.Fatalf("%s/%s: binary round-trip lost fields:\nwant: %v\n got: %v",
					contract, c.Name, m, parsed)
			}

			jsonBytes, err := protojson.Marshal(parsed)
			if err != nil {
				t.Fatalf("%s/%s: json marshal: %v", contract, c.Name, err)
			}
			viaJSON := newMessage(contract)
			if err := protojson.Unmarshal(jsonBytes, viaJSON); err != nil {
				t.Fatalf("%s/%s: json unmarshal: %v", contract, c.Name, err)
			}
			if !proto.Equal(m, viaJSON) {
				t.Fatalf("%s/%s: json round-trip lost fields:\nwant: %v\n got: %v",
					contract, c.Name, m, viaJSON)
			}
		}
	}
}

// TestUnknownFieldsPreserved proves a message with an unknown field, passed
// through a consumer that does not know it, still carries it (PLAN.md §3.4;
// TESTING.md §4.1). The unknown field is appended as raw wire bytes.
func TestUnknownFieldsPreserved(t *testing.T) {
	unknown := protowire.AppendVarint(
		protowire.AppendTag(nil, 99, protowire.VarintType),
		42,
	)
	for _, contract := range loadBearingContracts {
		for _, c := range loadPositiveFixtures(t, contract) {
			m := parsePositiveFixture(t, contract, c)
			known, err := proto.Marshal(m)
			if err != nil {
				t.Fatalf("%s/%s: marshal: %v", contract, c.Name, err)
			}

			withUnknown := append(append([]byte{}, known...), unknown...)
			parsed := newMessage(contract)
			if err := proto.Unmarshal(withUnknown, parsed); err != nil {
				t.Fatalf("%s/%s: unmarshal with unknown: %v", contract, c.Name, err)
			}
			reEncoded, err := proto.Marshal(parsed)
			if err != nil {
				t.Fatalf("%s/%s: re-marshal with unknown: %v", contract, c.Name, err)
			}
			// Compare semantically: protobuf maps do not guarantee a
			// deterministic wire order, so byte-for-byte equality does not
			// hold for messages that contain maps.  Compare against the
			// message that already carries the unknown field (parsed), not
			// the original fixture message (m) which lacks it.
			reParsed := newMessage(contract)
			if err := proto.Unmarshal(reEncoded, reParsed); err != nil {
				t.Fatalf("%s/%s: unmarshal re-encoded: %v", contract, c.Name, err)
			}
			if !proto.Equal(parsed, reParsed) {
				t.Fatalf("%s/%s: round-trip lost fields:\nparsed:  %v\nreEncoded: %v",
					contract, c.Name, parsed, reParsed)
			}
			// Belt-and-suspenders: verify the unknown field survived
			// the round-trip by scanning the wire bytes for tag 99
			// (varint, value 42).
			if !hasUnknownField(reEncoded, 99, 42) {
				t.Fatalf("%s/%s: unknown field not preserved:\n input: %x\noutput: %x",
					contract, c.Name, withUnknown, reEncoded)
			}

			jsonBytes, err := protojson.Marshal(parsed)
			if err != nil {
				t.Fatalf("%s/%s: json marshal with unknown: %v", contract, c.Name, err)
			}
			viaJSON := newMessage(contract)
			if err := protojson.Unmarshal(jsonBytes, viaJSON); err != nil {
				t.Fatalf("%s/%s: json unmarshal with unknown: %v", contract, c.Name, err)
			}
			if !proto.Equal(m, viaJSON) {
				t.Fatalf("%s/%s: json round-trip lost known fields alongside unknown:\nwant: %v\n got: %v",
					contract, c.Name, m, viaJSON)
			}
		}
	}
}

// TestStrictJSONRejectsUnknown proves the version boundary: JSON carrying a
// field v1 does not know is rejected, never silently dropped (PLAN.md §3.4).
func TestStrictJSONRejectsUnknown(t *testing.T) {
	for _, contract := range loadBearingContracts {
		c := loadPositiveFixtures(t, contract)[0]
		m := parsePositiveFixture(t, contract, c)
		jsonBytes, err := protojson.Marshal(m)
		if err != nil {
			t.Fatalf("%s: json marshal: %v", contract, err)
		}
		// Strip the trailing '}' and inject a field v1 cannot know.
		jsonWithUnknown := append([]byte{}, jsonBytes[:len(jsonBytes)-1]...)
		jsonWithUnknown = append(jsonWithUnknown, `,"some_future_field":"x"}`...)
		if err := protojson.Unmarshal(jsonWithUnknown, newMessage(contract)); err == nil {
			t.Fatalf("%s: strict JSON accepted an unknown field", contract)
		}
	}
}
