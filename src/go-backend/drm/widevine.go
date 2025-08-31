package drm

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	widevine "github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"

	"abcmovies/config"
	"abcmovies/utils"
)

type WidevineInterface interface {
	GetKeys() ([]*widevine.Key, error)
}

type Widevine struct{}

func (w *Widevine) init(encodedPssh string) (*widevine.Device, *widevine.PSSH, error) {
	var device *widevine.Device

	psshBytes, err := base64.RawStdEncoding.DecodeString(encodedPssh)

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

func (w *Widevine) GetKeys(psshData string, licenseUrl string, headers map[string]string) ([]string, error) {

	device, pssh, err := w.init(psshData)
	if err != nil {
		fmt.Println(err)
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
		cert, err := w.getServiceCert(licenseUrl)
		if err != nil {
			fmt.Println(fmt.Errorf("get service cert: %w", err))
			return nil, fmt.Errorf("get service cert: %w", err)
		}

		challenge, parseLicense, err = cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, true, cert)
		if err != nil {
			fmt.Println(fmt.Errorf("get license challenge: %w", err))
			return nil, fmt.Errorf("get license challenge: %w", err)
		}
	}

	// Send challenge to license server
	req, err := http.NewRequest(http.MethodPost, licenseUrl, io.NopCloser(bytes.NewReader(challenge)))
	if err != nil {
		fmt.Println(fmt.Errorf("couldn't create request: %w", err))
		return nil, fmt.Errorf("couldn't create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println(fmt.Errorf("request license: %w", err))
		return nil, fmt.Errorf("request license: %w", err)
	}
	defer resp.Body.Close()

	license, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(fmt.Errorf("read resp: %w", err))
		return nil, fmt.Errorf("read resp: %w", err)
	}

	// Parse license
	keys, err := parseLicense(license)
	if err != nil {
		fmt.Println(fmt.Errorf("parse license: %w", err))
		return nil, fmt.Errorf("parse license: %w", err)
	}

	if len(keys) == 0 {
		fmt.Println(fmt.Errorf("no keys: %w", err))
		return nil, fmt.Errorf("no keys: %w", err)
	}

	var formattedKeys []string

	for _, key := range keys {
		if key.Type == widevinepb.License_KeyContainer_CONTENT {
			formattedKeys = append(formattedKeys, fmt.Sprintf("%s:%s", hex.EncodeToString(key.ID), hex.EncodeToString(key.Key)))
		}
	}

	fmt.Println("Found keys:", formattedKeys)

	return formattedKeys, nil
}

func (w *Widevine) getServiceCert(licenseUrl string) (*widevinepb.DrmCertificate, error) {
	req, err := http.NewRequest(http.MethodPost, licenseUrl, io.NopCloser(bytes.NewReader(widevine.ServiceCertificateRequest)))
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
