package config

const (

	// Widevine

	CLIENT_ID_FILENAME   string = "client_id.bin"
	PRIVATE_KEY_FILENAME string = "private_key.pem"
	WVD_FILENAME         string = "device.wvd"

	WIDEVINE_UUID string = "EDEF8BA9-79D6-4ACE-A3C8-27DCD51D21ED"

	WIDEVINE_PSSH_PART_1 string = "000000387073736800000000edef8ba979d64acea3c827dcd51d21ed000000181210"
	WIDEVINE_PSSH_PART_3 string = "48e3dc959b06"

	WIDEVINE_PSSH_MIN_LEN int = 20
	WIDEVINE_PSSH_MAX_LEN int = 220
)
