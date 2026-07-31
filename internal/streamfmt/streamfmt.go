package streamfmt

import "github.com/nem-git/abcmovies/internal/oas"

func ShortName(f oas.StreamEncodingFormat) string {
	switch f {
	case oas.StreamEncodingFormatApplicationDashXML:
		return "dash"
	case oas.StreamEncodingFormatApplicationVndAppleMpegurl:
		return "hls"
	case oas.StreamEncodingFormatVideoMP4:
		return "mp4"
	default:
		return string(f)
	}
}

func Label(f oas.StreamEncodingFormat) string {
	switch f {
	case oas.StreamEncodingFormatApplicationDashXML:
		return "DASH"
	case oas.StreamEncodingFormatApplicationVndAppleMpegurl:
		return "HLS"
	case oas.StreamEncodingFormatVideoMP4:
		return "MP4"
	default:
		return string(f)
	}
}

func BadgeColor(f oas.StreamEncodingFormat) string {
	switch f {
	case oas.StreamEncodingFormatApplicationDashXML:
		return "bg-green-900/60 text-green-300"
	case oas.StreamEncodingFormatApplicationVndAppleMpegurl:
		return "bg-blue-900/60 text-blue-300"
	case oas.StreamEncodingFormatVideoMP4:
		return "bg-purple-900/60 text-purple-300"
	default:
		return "bg-gray-700 text-gray-300"
	}
}
