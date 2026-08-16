package drm

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/Eyevinn/mp4ff/mp4"
)

// System IDs of the DRM schemes this package supports.
var (
	widevineSystemID  = mustUUID("edef8ba979d64acea3c827dcd51d21ed")
	playReadySystemID = mustUUID("9a04f07998404286ab92e65be0885f95")
	clearKeySystemID  = mustUUID("e2719d58a985b3c9781ab030af78d8e5")
	fairPlaySystemID  = mustUUID("94ce86fb07ff4f43adb893d2fa968ca2")
)

// mustUUID decodes a 32-char hex string into a UUID.
func mustUUID(s string) mp4.UUID {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return mp4.UUID(b)
}

// protectInfo is the per-stream protection data extracted from an init segment.
type protectInfo struct {
	// ContentID is the content identifier from the first Widevine or PlayReady PSSH.
	ContentID []byte
	// KeyID is the default KID from the first tenc box (may be the blank KID).
	KeyID []byte
}

// extractInitProtectInfo scans an init segment for Widevine/PlayReady PSSH
// boxes and tenc boxes, returning the content ID and default key ID.
// Mirrors the resolution order used by maya and Devine:
// Widevine PSSH content ID first, then PlayReady PRO content ID, then tenc KID.
func extractInitProtectInfo(init *mp4.InitSegment) (*protectInfo, error) {
	if init == nil || init.Moov == nil {
		return nil, errors.New("drm: init segment has no moov box")
	}
	info := &protectInfo{}

	// 1. Content ID from Widevine PSSH.
	for _, pssh := range init.Moov.Psshs {
		if pssh.SystemID.Equal(widevineSystemID) {
			data, err := DecodeWidevinePSSH(pssh.Data)
			if err == nil && len(data.ContentID) > 0 {
				info.ContentID = data.ContentID
				break
			}
		}
	}

	// 2. Content ID from PlayReady PRO (fallback for PlayReady-only titles).
	if info.ContentID == nil {
		for _, pssh := range init.Moov.Psshs {
			if pssh.SystemID.Equal(playReadySystemID) {
				pro, err := decodePlayReadyPRO(pssh.Data)
				if err == nil && len(pro) > 0 {
					info.ContentID = pro
					break
				}
			}
		}
	}

	// 3. Default KID from the first tenc box.
	for _, trak := range init.Moov.Traks {
		stsd := trak.Mdia.Minf.Stbl.Stsd
		for _, child := range stsd.Children {
			var sinf *mp4.SinfBox
			switch t := child.(type) {
			case *mp4.VisualSampleEntryBox:
				sinf = t.Sinf
			case *mp4.AudioSampleEntryBox:
				sinf = t.Sinf
			default:
				continue
			}
			if sinf == nil || sinf.Schi == nil || sinf.Schi.Tenc == nil {
				continue
			}
			tenc := sinf.Schi.Tenc
			if tenc.DefaultIsProtected == 0 {
				continue
			}
			kid := tenc.DefaultKID
			if !isBlankKID(kid) {
				info.KeyID = kid
				return info, nil
			}
			if info.KeyID == nil {
				info.KeyID = kid
			}
		}
	}

	return info, nil
}

// isBlankKID reports whether a KID is all zeros (services that use a
// non-standard KID; per ANSWERS.md these decrypt with the license key anyway).
func isBlankKID(kid mp4.UUID) bool {
	for _, b := range kid {
		if b != 0 {
			return false
		}
	}
	return true
}

// kidsFromInit returns the default KIDs of all encrypted tracks in the init
// segment, deduplicated.
func kidsFromInit(init *mp4.InitSegment) []KID {
	var kids []KID
	seen := map[KID]bool{}
	for _, trak := range init.Moov.Traks {
		stsd := trak.Mdia.Minf.Stbl.Stsd
		for _, child := range stsd.Children {
			var sinf *mp4.SinfBox
			switch t := child.(type) {
			case *mp4.VisualSampleEntryBox:
				sinf = t.Sinf
			case *mp4.AudioSampleEntryBox:
				sinf = t.Sinf
			default:
				continue
			}
			if sinf == nil || sinf.Schi == nil || sinf.Schi.Tenc == nil {
				continue
			}
			if sinf.Schi.Tenc.DefaultIsProtected == 0 {
				continue
			}
			var kid KID
			copy(kid[:], sinf.Schi.Tenc.DefaultKID)
			if !seen[kid] {
				seen[kid] = true
				kids = append(kids, kid)
			}
		}
	}
	return kids
}

// psshPayloads returns the raw payloads of the given scheme's PSSH boxes in an
// init segment. Useful when the pssh box must be reconstructed (e.g. ClearKey).
func psshPayloads(init *mp4.InitSegment) [][]byte {
	var out [][]byte
	for _, pssh := range init.Moov.Psshs {
		out = append(out, pssh.Data)
	}
	return out
}

// decodePlayReadyPRO parses the PRO XML header out of a PlayReady PSSH payload,
// returning the content ID if present.
func decodePlayReadyPRO(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("drm: empty playready pssh")
	}
	// PlayReady PSSH payloads in the MP4 box are raw PRO XML (not a system
	// header). Delegate to the diana playReady package when wired.
	return playReadyContentID(data)
}

// schemeFromInit returns the DRM scheme identified by the first matching PSSH
// system ID in the init segment, or "" when none match.
func schemeFromInit(init *mp4.InitSegment) Scheme {
	if init == nil || init.Moov == nil {
		return ""
	}
	for _, pssh := range init.Moov.Psshs {
		switch {
		case pssh.SystemID.Equal(widevineSystemID):
			return SchemeWidevine
		case pssh.SystemID.Equal(playReadySystemID):
			return SchemePlayReady
		case pssh.SystemID.Equal(clearKeySystemID):
			return SchemeClearKey
		}
	}
	return ""
}

// psshPayloadForScheme returns the raw payload of the first PSSH box matching
// the scheme's system ID in the init segment.
func psshPayloadForScheme(init *mp4.InitSegment, scheme Scheme) []byte {
	if init == nil || init.Moov == nil {
		return nil
	}
	var target mp4.UUID
	switch scheme {
	case SchemeWidevine:
		target = widevineSystemID
	case SchemePlayReady:
		target = playReadySystemID
	case SchemeClearKey:
		target = clearKeySystemID
	default:
		return nil
	}
	for _, pssh := range init.Moov.Psshs {
		if pssh.SystemID.Equal(target) {
			return pssh.Data
		}
	}
	return nil
}
