package gobackend

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	widevine "github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
)

type WidevineDataInterface interface {
	GetKeys() ([]*widevine.Key, error)
}

type WidevineData struct {
	device     *widevine.Device
	pssh       *widevine.PSSH
	licenseUrl string
}

func (widevineData *WidevineData) init(psshData []byte) error {
	var err error

	// Parse PSSH
	widevineData.pssh, err = widevine.NewPSSH(psshData)
	if err != nil {
		return fmt.Errorf("parse pssh: %w", err)
	}

	wvd, err := os.Open(WVD_FILENAME)
	if err == nil {
		// Tries to get the wvd, if fails fallback to client_id and private_key
		widevineData.device, err = widevine.NewDevice(
			widevine.FromWVD(wvd),
		)
	}
	if err != nil {
		clientId, err := openFile(CLIENT_ID_FILENAME)
		if err != nil {
			return fmt.Errorf("getting client ID: %w", err)
		}

		privateKey, err := openFile(PRIVATE_KEY_FILENAME)
		if err != nil {
			return fmt.Errorf("getting private key: %w", err)
		}

		widevineData.device, err = widevine.NewDevice(
			widevine.FromRaw(clientId, privateKey),
		)
		if err != nil {
			return fmt.Errorf("create device: %w", err)
		}
	}

	return nil
}

func (widevineData *WidevineData) GetKeys(psshData []byte) ([]*widevine.Key, error) {
	err := widevineData.init(psshData)
	if err != nil {
		return nil, err
	}

	// Create CDM
	cdm := widevine.NewCDM(widevineData.device)

	var (
		challenge    []byte
		parseLicense func(b []byte) ([]*widevine.Key, error)
	)

	// Get license challenge
	challenge, parseLicense, err = cdm.GetLicenseChallenge(widevineData.pssh, widevinepb.LicenseType_AUTOMATIC, false)
	if err != nil {

		// Or use privacy mode
		cert, err := widevineData.getServiceCert()
		if err != nil {
			return nil, fmt.Errorf("get service cert: %w", err)
		}

		challenge, parseLicense, err = cdm.GetLicenseChallenge(widevineData.pssh, widevinepb.LicenseType_AUTOMATIC, true, cert)
		if err != nil {
			return nil, fmt.Errorf("get license challenge: %w", err)
		}
	}

	// Send challenge to license server
	req, err := http.NewRequest(http.MethodPost, widevineData.licenseUrl, io.NopCloser(bytes.NewReader(challenge)))
	if err != nil {
		return nil, fmt.Errorf("couldn't create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request license: %w", err)
	}
	defer resp.Body.Close()

	license, err := io.ReadAll(resp.Body)
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

	return keys, nil
}

func (widevineData *WidevineData) getServiceCert() (*widevinepb.DrmCertificate, error) {
	req, err := http.NewRequest(http.MethodPost, widevineData.licenseUrl, io.NopCloser(bytes.NewReader(widevine.ServiceCertificateRequest)))
	if err != nil {
		return nil, fmt.Errorf("couldn't create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request service cert: %w", err)
	}
	defer resp.Body.Close()

	serviceCert, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	cert, err := widevine.ParseServiceCert(serviceCert)
	if err != nil {
		return nil, fmt.Errorf("parse service cert: %w", err)
	}

	return cert, nil
}
