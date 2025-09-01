package config

const (
	// Files
	CDM_PATH string = "../../cdm/widevine"

	// Widevine
	CLIENT_ID_FILENAME   string = "client_id.bin"
	PRIVATE_KEY_FILENAME string = "private_key.pem"
	WVD_FILENAME         string = "device.wvd"

	// Dash
	DASH_INIT_URL_PREFIX  string = "dash/init"
	DASH_MEDIA_URL_PREFIX string = "dash/media"

	// Regex
	RE_DASH_MANIFEST_PLAYREADY string = "xmlns:mspr=\"urn:microsoft:playready\""
	RE_DASH_MANIFEST_CENC      string = "xmlns:cenc=\"urn:mpeg:cenc:2013\""
	RE_DASH_MANIFEST_CLEARKEY  string = "xmlns:ck=\"http://dashif.org/guidelines/clearKey\"" // Unsure
)
