package keys

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	widevine "github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"

	"github.com/nem-git/abcmovies/config"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Get(psshData string, licenseUrl string, headers map[string]string) ([]string, error) {

	device, pssh, err := initSegment(psshData)
	if err != nil {
		return nil, err
	}

	// Create CDM
	cdm := widevine.NewCDM(device)

	var (
		challenge    []byte
		parseLicense func(b []byte) ([]*widevine.Key, error)
	)

	// Get license challenge
	challenge, parseLicense, err = cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, false)
	if err != nil {

		// Or use privacy mode
		cert, err := getServiceCert(licenseUrl)
		if err != nil {
			return nil, fmt.Errorf("get service cert: %w", err)
		}

		challenge, parseLicense, err = cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, true, cert)
		if err != nil {
			return nil, fmt.Errorf("get license challenge: %w", err)
		}
	}

	// Send challenge to license server
	body, err := utils.Post(licenseUrl, headers, challenge)
	if err != nil {
		return nil, fmt.Errorf("couldn't make license request: %w", err)
	}

	license, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read resp: %w", err)
	}

	// Parse license
	keys, err := parseLicense(license)
	if err != nil {
		return nil, fmt.Errorf("parse license: %w", err)
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("no keys: %w", err)
	}

	var formattedKeys []string

	for _, key := range keys {
		if key.Type == widevinepb.License_KeyContainer_CONTENT {
			formattedKeys = append(formattedKeys, fmt.Sprintf("%s:%s", hex.EncodeToString(key.ID), hex.EncodeToString(key.Key)))
		}
	}

	return formattedKeys, nil
}

func getServiceCert(licenseUrl string) (*widevinepb.DrmCertificate, error) {

	body, err := utils.Post(licenseUrl, nil, widevine.ServiceCertificateRequest)
	if err != nil {
		return nil, fmt.Errorf("error while making service cert request: %w", err)
	}

	serviceCert, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("error while reading service cert response: %w", err)
	}

	cert, err := widevine.ParseServiceCert(serviceCert)
	if err != nil {
		return nil, fmt.Errorf("parsing service cert failed: %w", err)
	}

	return cert, nil
}

func initSegment(encodedPssh string) (*widevine.Device, *widevine.PSSH, error) {
	var device *widevine.Device

	psshBytes, err := base64.StdEncoding.DecodeString(encodedPssh)
	if err != nil {
		return &widevine.Device{}, &widevine.PSSH{}, fmt.Errorf("base64 decoding pssh %v: %w", psshBytes, err)
	}

	// Parse PSSH
	pssh, err := widevine.NewPSSH(psshBytes)
	if err != nil {
		return &widevine.Device{}, &widevine.PSSH{}, fmt.Errorf("parse pssh %v: %w", encodedPssh, err)
	}

	wvd, err := utils.OpenCdmFile(config.WVD_FILENAME)
	if err == nil {
		// Tries to get the wvd, if fails fallback to client_id and private_key
		device, err = widevine.NewDevice(
			widevine.FromWVD(wvd),
		)
	}
	if err != nil {
		clientId, err := utils.GetCdmFile(config.CLIENT_ID_FILENAME)
		if err != nil {
			return &widevine.Device{}, &widevine.PSSH{}, fmt.Errorf("getting client ID: %w", err)
		}

		privateKey, err := utils.GetCdmFile(config.PRIVATE_KEY_FILENAME)
		if err != nil {
			return &widevine.Device{}, &widevine.PSSH{}, fmt.Errorf("getting private key: %w", err)
		}

		device, err = widevine.NewDevice(
			widevine.FromRaw(clientId, privateKey),
		)
		if err != nil {
			return &widevine.Device{}, &widevine.PSSH{}, fmt.Errorf("create device: %w", err)
		}
	}

	return device, pssh, nil
}
