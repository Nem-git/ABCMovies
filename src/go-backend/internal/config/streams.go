package config

const (
	STREAM_DASH_TYPE   string = "dash"
	STREAM_HLS_TYPE    string = "hls"
	STREAM_MP4_TYPE    string = "mp4"
	STREAM_SMOOTH_TYPE string = "smooth"

	STREAM_DASH_FILE_NAME   string = "manifest.mpd"
	STREAM_HLS_FILE_NAME    string = "master.m3u"
	STREAM_MP4_FILE_NAME    string = "video.mp4"
	STREAM_SMOOTH_FILE_NAME string = "Manifest"
)

var (
	STREAM_TYPE_TO_FILE_NAME = map[string]string{
		STREAM_DASH_TYPE:   STREAM_DASH_FILE_NAME,
		STREAM_HLS_TYPE:    STREAM_HLS_FILE_NAME,
		STREAM_MP4_TYPE:    STREAM_MP4_FILE_NAME,
		STREAM_SMOOTH_TYPE: STREAM_SMOOTH_FILE_NAME,
	}
)
