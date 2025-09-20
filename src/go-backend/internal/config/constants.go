package config

const (
	// Files
	CDM_PATH string = "../../../../bin/widevine"

	// Widevine & Playready
	CENC_SCHEME_ID = "MPEG:DASH:MP4PROTECTION:2011"

	// Widevine
	CLIENT_ID_FILENAME   string = "client_id.bin"
	PRIVATE_KEY_FILENAME string = "private_key.pem"
	WVD_FILENAME         string = "device.wvd"

	WIDEVINE_UUID string = "EDEF8BA9-79D6-4ACE-A3C8-27DCD51D21ED"

	WIDEVINE_PSSH_PART_1 string = "000000387073736800000000edef8ba979d64acea3c827dcd51d21ed000000181210"
	WIDEVINE_PSSH_PART_3 string = "48e3dc959b06"

	WIDEVINE_PSSH_MIN_LEN int = 20
	WIDEVINE_PSSH_MAX_LEN int = 220

	// Playready
	PLAYREADY_UUID string = "9A04F079-9840-4286-AB92-E65BE0885F95"

	// Dash
	DASH_INIT_URL_PREFIX  string = "dash/init"
	DASH_MEDIA_URL_PREFIX string = "dash/media"

	// Regex
	RE_DASH_MANIFEST_PLAYREADY string = "xmlns:mspr=\"urn:microsoft:playready\""
	RE_DASH_MANIFEST_CENC      string = "xmlns:cenc=\"urn:mpeg:cenc:2013\""
	RE_DASH_MANIFEST_CLEARKEY  string = "xmlns:ck=\"http://dashif.org/guidelines/clearKey\"" // Unsure
)
