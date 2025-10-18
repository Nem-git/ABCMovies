package config

const (
	DASH_CONTENT_TYPE   string = "application/dash+xml"
	HLS_CONTENT_TYPE    string = "application/x-mpegURL"
	SMOOTH_CONTENT_TYPE string = "text/xml"

	MP4_CONTENT_TYPE string = "video/mp4"
)

var (
	CONTENT_TYPE_TO_FILE_NAME = map[string]string{
		STREAM_DASH_TYPE:   DASH_CONTENT_TYPE,
		STREAM_HLS_TYPE:    HLS_CONTENT_TYPE,
		STREAM_MP4_TYPE:    SMOOTH_CONTENT_TYPE,
		STREAM_SMOOTH_TYPE: MP4_CONTENT_TYPE,
	}
)
