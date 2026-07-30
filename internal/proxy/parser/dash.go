package parser

import (
	"github.com/Eyevinn/dash-mpd/mpd"
)

// DASHParser handles DASH MPD parsing.
type DASHParser struct{}

// Read parses an MPD from a string and sets parent pointers.
func (p *DASHParser) Read(data []byte) (*mpd.MPD, error) {
	m, err := mpd.ReadFromString(string(data))
	if err != nil {
		return nil, err
	}
	m.SetParents()
	return m, nil
}

// Write encodes an MPD to bytes.
func (p *DASHParser) Write(m *mpd.MPD) ([]byte, error) {
	s, err := m.WriteToString("", false)
	if err != nil {
		return nil, err
	}
	return []byte(s), nil
}

// EffectiveSegmentTemplate walks the hierarchy to find the effective SegmentTemplate
// for a Representation: Representation → AdaptationSet → Period.
func EffectiveSegmentTemplate(rep *mpd.RepresentationType) *mpd.SegmentTemplateType {
	if st := rep.GetSegmentTemplate(); st != nil {
		return st
	}
	a := rep.Parent()
	if a == nil {
		return nil
	}
	if st := a.GetSegmentTemplate(); st != nil {
		return st
	}
	p := a.Parent()
	if p == nil {
		return nil
	}
	return p.SegmentTemplate
}
