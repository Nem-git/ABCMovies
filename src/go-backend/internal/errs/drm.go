package errs

import "errors"

var (
	// Widevine

	// Keys
	ErrEmptyWidevinePSSH             = errors.New("no PSSH given")
	ErrWidevineDecodePSSH            = errors.New("couldn't base64 decode PSSH")
	ErrWidevineParsePSSH             = errors.New("couldn't parse PSSH")
	ErrWidevineOpeningClientIDFile   = errors.New("couldn't open client ID file")
	ErrWidevineOpeningPrivateKeyFile = errors.New("couldn't open private key file")
	ErrWidevineCreatingDevice        = errors.New("couldn't create device")

	ErrWidevineLicenseServiceCertificate            = errors.New("couldn't get license service certificate")
	ErrWidevineLicenseServiceCertificateRequest     = errors.New("couldn't make service certificate request")
	ErrWidevineLicenseServiceCertificateRequestBody = errors.New("couldn't get license request body")
	ErrWidevineLicenseServiceCertificateParsing     = errors.New("couldn't parse service certificate to cert")

	ErrEmptyWidevineLicenseURL          = errors.New("no license URL given")
	ErrWidevineLicenseChallenge         = errors.New("couldn't get license challenge")
	ErrWidevineLicenseRequest           = errors.New("couldn't make license request")
	ErrWidevineLicenseResponseBody      = errors.New("couldn't parse license response body as bytes")
	ErrWidevineLicenseResponseWrongMime = errors.New("license response body mime not application/json")
	ErrWidevineLicenseResponseToLicense = errors.New("couldn't parse license response body as license")
	ErrWidevineLicenseResponseKeys      = errors.New("no keys found in license response")

	ErrEmptyWidevinePlaylistURL = errors.New("no playlist URL given")

	ErrEmptyWidevineKeys = errors.New("no widevine decryption keys given")
)
