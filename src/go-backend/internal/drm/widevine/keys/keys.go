package keys

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	widevine "github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
	"github.com/nem-git/abcmovies/internal/storage/cache/controller"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Get(encodedPSSH string, url string, headers map[string]string, controller *controller.CacheController) ([]string, error) {

	// Try to retrieve it in the db
	if controller != nil {
		keys, err := (*controller).ReadCollection(encodedPSSH)
		if err != nil {
			return nil, err
		}

		return keys, nil
	}

	device, pssh, err := decode(encodedPSSH)
	if err != nil {
		return nil, err
	}

	// Create CDM
	cdm := widevine.NewCDM(device)

	// Get license challenge
	challenge, parseLicense, err := cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, false)
	if err != nil {

		// Or use privacy mode
		cert, err := getServiceCert(url)
		if err != nil {
			return nil, errs.ErrWidevineLicenseServiceCertificate
		}

		challenge, parseLicense, err = cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, true, cert)
		if err != nil {
			return nil, errs.ErrWidevineLicenseChallenge
		}
	}

	// Send challenge to license server
	body, err := utils.Post(url, headers, challenge)
	if err != nil {
		return nil, errs.ErrWidevineLicenseRequest
	}
	defer body.Close()

	license, err := io.ReadAll(body)
	if err != nil {
		return nil, errs.ErrWidevineLicenseResponseBody
	}

	mimeType := http.DetectContentType(license)

	if mimeType != "application/octet-stream" {
		return nil, errs.ErrWidevineLicenseResponseWrongMime
	}

	// Parse license
	keys, err := parseLicense(license)
	if err != nil {
		return nil, errs.ErrWidevineLicenseResponseToLicense
	}

	if len(keys) == 0 {
		return nil, errs.ErrWidevineLicenseResponseKeys
	}

	var formattedKeys []string

	for _, key := range keys {
		if key.Type == widevinepb.License_KeyContainer_CONTENT {
			formattedKeys = append(formattedKeys, fmt.Sprintf("%s:%s", hex.EncodeToString(key.ID), hex.EncodeToString(key.Key)))
		}
	}

	if len(formattedKeys) == 0 {
		return nil, errs.ErrWidevineLicenseResponseKeys
	}

	return formattedKeys, nil
}

func getServiceCert(licenseUrl string) (*widevinepb.DrmCertificate, error) {

	body, err := utils.Post(licenseUrl, nil, widevine.ServiceCertificateRequest)
	if err != nil {
		return nil, errs.ErrWidevineLicenseServiceCertificateRequest
	}
	defer body.Close()

	serviceCert, err := io.ReadAll(body)
	if err != nil {
		return nil, errs.ErrWidevineLicenseServiceCertificateRequestBody
	}

	cert, err := widevine.ParseServiceCert(serviceCert)
	if err != nil {
		return nil, errs.ErrWidevineLicenseServiceCertificateParsing
	}

	return cert, nil
}

func decode(encodedPSSH string) (*widevine.Device, *widevine.PSSH, error) {
	var device *widevine.Device

	psshBytes, err := base64.StdEncoding.DecodeString(encodedPSSH)
	if err != nil {
		return &widevine.Device{}, &widevine.PSSH{}, errs.ErrWidevineDecodePSSH
	}

	// Parse PSSH
	pssh, err := widevine.NewPSSH(psshBytes)
	if err != nil {
		return &widevine.Device{}, &widevine.PSSH{}, errs.ErrWidevineParsePSSH
	}

	wvd, err := utils.OpenCdmFile(config.WVD_FILENAME)
	if err == nil {
		// Tries to get the wvd, if fails fallback to client_id and private_key
		device, err = widevine.NewDevice(
			widevine.FromWVD(wvd),
		)
	}
	if err != nil {
		clientID, err := utils.GetCdmFile(config.CLIENT_ID_FILENAME)
		if err != nil {
			return &widevine.Device{}, &widevine.PSSH{}, errs.ErrWidevineOpeningClientIDFile
		}

		privateKey, err := utils.GetCdmFile(config.PRIVATE_KEY_FILENAME)
		if err != nil {
			return &widevine.Device{}, &widevine.PSSH{}, errs.ErrWidevineOpeningPrivateKeyFile
		}

		device, err = widevine.NewDevice(
			widevine.FromRaw(clientID, privateKey),
		)
		if err != nil {
			return &widevine.Device{}, &widevine.PSSH{}, errs.ErrWidevineCreatingDevice
		}
	}

	return device, pssh, nil
}
