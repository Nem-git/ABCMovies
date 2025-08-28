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

type WidevineData struct {
	device     *widevine.Device
	clientId   []byte
	privateKey []byte
	wvd        []byte
	psshData   []byte

	decryptionKeys []*widevine.Key
}

func (widevineData *WidevineData) init(psshData []byte) error {
	var err error

	widevineData.psshData = psshData

	wvd, err := os.Open(WVD_FILENAME)

	// Tries to get the wvd, if fails fallback to client_id and private_key
	widevineData.device, err = widevine.NewDevice(
		widevine.FromWVD(wvd),
	)
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

func (widevineData *WidevineData) getWidevineKeys() ([]*widevine.Key, error) {

	var err error

	keys, err := getKeys()
	if err != nil {
		panic(err)
	}
	if len(keys) == 0 {
		panic("no keys")
	}

	for _, key := range keys {
		fmt.Printf("type: %s, id: %x, key: %x\n", key.Type, key.ID, key.Key)
	}

	err = widevine.DecryptMP4Auto(bytes.NewBufferString("encrypted data"),
		keys, io.Discard)
	if err != nil {
		panic(err)
	}

	return keys, nil
}

func getKeys(psshData []byte, wv []byte, clientId []byte, privateKey []byte) ([]*widevine.Key, error) {
	// Create device from raw data or from wvd file
	device, err := widevine.NewDevice(
		widevine.FromRaw(clientId, privateKey),
		// widevine.FromWVD(bytes.NewReader([]byte("baz"))),
	)
	if err != nil {
		return nil, fmt.Errorf("create device: %w", err)
	}
	// Create CDM
	cdm := widevine.NewCDM(device)

	// Parse PSSH
	pssh, err := widevine.NewPSSH(psshData)
	if err != nil {
		return nil, fmt.Errorf("parse pssh: %w", err)
	}

	// Get license challenge
	challenge, parseLicense, err := cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, false)
	if err != nil {

		// Or use privacy mode
		cert, err := getServiceCert()
		if err != nil {
			return nil, fmt.Errorf("get service cert: %w", err)
		}
		challenge, parseLicense, err = cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, true, cert)
		if err != nil {
			return nil, fmt.Errorf("get license challenge: %w", err)
		}

		return nil, fmt.Errorf("get license challenge: %w", err)
	}

	// Send challenge to license server
	resp, err := http.DefaultClient.Do(&http.Request{Body: io.NopCloser(bytes.NewReader(challenge))})
	if err != nil {
		return nil, fmt.Errorf("request license: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	license, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read resp: %w", err)
	}

	// Parse license
	keys, err := parseLicense(license)
	if err != nil {
		return nil, fmt.Errorf("parse license: %w", err)
	}

	return keys, nil
}

func getServiceCert() (*widevinepb.DrmCertificate, error) {
	resp, err := http.DefaultClient.Do(&http.Request{Body: io.NopCloser(bytes.NewReader(widevine.ServiceCertificateRequest))})
	if err != nil {
		return nil, fmt.Errorf("request service cert: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
