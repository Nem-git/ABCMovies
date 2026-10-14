package delivery

import (
	"fmt"
	"strings"

	corev1 "github.com/nem-git/abcmovies/core/gen/abcmovies/core/v1"
)

// BuildHLS renders an HLS master playlist from the session's tracks,
// translating our track descriptors into the standard EXT-X-MEDIA and
// EXT-X-STREAM-INF attributes (bandwidth, dimensions, codecs, resolution,
// frame-rate, video-range, language, channels, forced/caption flags, and the
// standard audio/subtitle roles). urlFor returns the relay URL that serves one
// track's bytes; the manifest references those URLs, so a player opens the
// manifest and pulls the renditions it selects.
//
// A track with no location (a carried-in audio/subtitle inside a WHOLE_MUX
// container, or a burned-in subtitle) is skipped: it has no independent bytes,
// so the player reads it off the carrier track's delivery.
//
// This is the engine's manifest builder (PLAN.md §6.1): the Package pipeline
// step owns what a manifest looks like, so every sink — device or media
// server — can be packaged, and the builder never imports an adapter.
func BuildHLS(tracks []*corev1.Track, urlFor func(trackID string) (string, error)) (string, error) {
	var videos, audios, subs []*corev1.Track
	for _, tr := range tracks {
		if tr.GetDelivery() == nil || len(tr.GetDelivery().GetLocations()) == 0 {
			continue
		}
		switch tr.GetMedia().(type) {
		case *corev1.Track_Video:
			videos = append(videos, tr)
		case *corev1.Track_Audio:
			audios = append(audios, tr)
		case *corev1.Track_Subtitle:
			subs = append(subs, tr)
		}
	}

	var b strings.Builder
	b.WriteString("#EXTM3U\n")
	b.WriteString("#EXT-X-VERSION:3\n")

	for _, tr := range audios {
		u, err := urlFor(tr.GetId())
		if err != nil {
			return "", err
		}
		a := tr.GetAudio()
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=%q,LANGUAGE=%q,NAME=%q,URI=%q\n",
			"audio", a.GetLanguage(), a.GetLanguage(), u)
	}

	for _, tr := range subs {
		u, err := urlFor(tr.GetId())
		if err != nil {
			return "", err
		}
		s := tr.GetSubtitle()
		fmt.Fprintf(&b, "#EXT-X-MEDIA:TYPE=SUBTITLES,GROUP-ID=%q,LANGUAGE=%q,NAME=%q,URI=%q\n",
			"subs", s.GetLanguage(), subtitleRole(s.GetRole()), u)
	}

	for _, tr := range videos {
		u, err := urlFor(tr.GetId())
		if err != nil {
			return "", err
		}
		v := tr.GetVideo()
		fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,CODECS=%q\n",
			v.GetBitrate(), v.GetWidth(), v.GetHeight(), v.GetCodec())
		b.WriteString(u)
		b.WriteString("\n")
	}

	return b.String(), nil
}

// subtitleRole names a subtitle track with its standard role, defaulting to
// "subtitle" when unset (PLAN.md §6.2: the standard subtitle-role vocabulary).
func subtitleRole(r corev1.SubtitleRole) string {
	switch r {
	case corev1.SubtitleRole_SUBTITLE_ROLE_SDH:
		return "sdh"
	case corev1.SubtitleRole_SUBTITLE_ROLE_SIGNS_SONGS:
		return "signs-songs"
	case corev1.SubtitleRole_SUBTITLE_ROLE_EASYREADER:
		return "easyreader"
	case corev1.SubtitleRole_SUBTITLE_ROLE_COMMENTARY:
		return "commentary"
	default:
		return "subtitle"
	}
}
