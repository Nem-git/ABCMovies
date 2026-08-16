package drm

import (
	"errors"

	"41.neocities.org/diana/playReady"
	"41.neocities.org/diana/widevine"
)

// widevinePSSH is the decoded Widevine protection header.
type widevinePSSH struct {
	KeyIDs    [][]byte
	ContentID []byte
}

// decodeWidevinePSSH decodes the pssh payload with diana's protobuf parser.
func decodeWidevinePSSH(data []byte) (*widevinePSSH, error) {
	p, err := widevine.DecodePsshData(data)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New("drm: nil widevine pssh")
	}
	return &widevinePSSH{KeyIDs: p.KeyIds, ContentID: p.ContentId}, nil
}

// playReadyContentID extracts the content ID from a PlayReady PRO XML payload
// (the CUSTOMATTRIBUTES CONTENTID used by 9c9media-style license servers).
func playReadyContentID(data []byte) ([]byte, error) {
	pro, err := playReady.ParsePro(data)
	if err != nil {
		return nil, err
	}
	if pro == nil || pro.Data.CustomAttributes == nil {
		return nil, nil
	}
	if id := pro.Data.CustomAttributes.ContentId; id != "" {
		return []byte(id), nil
	}
	return nil, nil
}
