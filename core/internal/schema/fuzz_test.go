package schema

import (
	"testing"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// FuzzBinaryParseLoadBearingContracts mutates the marshaled bytes of the
// positive fixtures for every load-bearing contract: parsing must reject
// cleanly or, when it succeeds, produce a message that re-marshals without
// error — never panic, never corrupt (TESTING.md §4.1).
func FuzzBinaryParseLoadBearingContracts(f *testing.F) {
	for _, contract := range loadBearingContracts {
		for _, c := range loadPositiveFixtures(f, contract) {
			m := parsePositiveFixture(f, contract, c)
			seed, err := proto.Marshal(m)
			if err != nil {
				f.Fatalf("%s/%s: seed marshal: %v", contract, c.Name, err)
			}
			f.Add(seed)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, contract := range loadBearingContracts {
			m := newMessage(contract)
			if err := proto.Unmarshal(data, m); err != nil {
				continue
			}
			if _, err := proto.Marshal(m); err != nil {
				t.Fatalf("%s: corrupted state after parse: %v", contract, err)
			}
		}
	})
}

// FuzzJSONParseLoadBearingContracts mutates the protojson text of the positive
// fixtures: JSON parsing must reject cleanly or produce a consistent message,
// never panic (TESTING.md §4.1).
func FuzzJSONParseLoadBearingContracts(f *testing.F) {
	for _, contract := range loadBearingContracts {
		for _, c := range loadPositiveFixtures(f, contract) {
			f.Add(string(c.Message))
		}
	}
	f.Fuzz(func(t *testing.T, text string) {
		for _, contract := range loadBearingContracts {
			m := newMessage(contract)
			if err := protojson.Unmarshal([]byte(text), m); err != nil {
				continue
			}
			if _, err := proto.Marshal(m); err != nil {
				t.Fatalf("%s: corrupted state after json parse: %v", contract, err)
			}
		}
	})
}
