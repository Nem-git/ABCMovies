package drm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	dianaWV "41.neocities.org/diana/widevine"
	gw "github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
)

// WidevineBackend selects the Widevine CDM implementation.
type WidevineBackend string

const (
	// WidevineDiana uses the diana pure-Go Widevine implementation (default).
	WidevineDiana WidevineBackend = "diana"
	// WidevineGoWidevine uses gw.
	WidevineGoWidevine WidevineBackend = "gowidevine"
)

// WidevineConfig configures the Widevine key provider.
type WidevineConfig struct {
	// Backend selects the CDM implementation (diana | gowidevine). Defaults to diana.
	Backend WidevineBackend
	// DeviceDir is the directory containing the raw device files
	// (device_client_id_blob + device_private_key) shared by both backends.
	DeviceDir string
	// WVD is an optional path to a .wvd file for the gowidevine backend.
	WVD string
	// Privacy enables service-certificate privacy mode on the gowidevine backend.
	Privacy bool
	// ServiceCertURL is the URL to fetch the service certificate from in privacy mode.
	ServiceCertURL string
	// Headers are extra headers for all license requests.
	Headers map[string]string
}

// WidevineProvider acquires Widevine keys. It implements KeyProvider.
type WidevineProvider struct {
	backend WidevineBackend
	cfg     WidevineConfig
	client  *licenseClient

	// diana device material (lazily loaded).
	clientID []byte
	privKey  []byte
	// gowidevine device.
	gwDevice *gw.Device
}

// NewWidevine returns a Widevine key provider from cfg.
func NewWidevine(cfg WidevineConfig) (*WidevineProvider, error) {
	p := &WidevineProvider{
		backend: cfg.Backend,
		cfg:     cfg,
		client:  newLicenseClient(),
	}
	if p.backend == "" {
		p.backend = WidevineDiana
	}
	if err := p.loadDevice(); err != nil {
		return nil, err
	}
	return p, nil
}

// Scheme implements KeyProvider.
func (p *WidevineProvider) Scheme() Scheme {
	return SchemeWidevine
}

// loadDevice loads the device material for the configured backend.
func (p *WidevineProvider) loadDevice() error {
	switch p.backend {
	case WidevineDiana:
		clientID, err := os.ReadFile(filepath.Join(p.cfg.DeviceDir, "device_client_id_blob"))
		if err != nil {
			return fmt.Errorf("%w: reading device_client_id_blob: %v", ErrDeviceMissing, err)
		}
		privKey, err := os.ReadFile(filepath.Join(p.cfg.DeviceDir, "device_private_key"))
		if err != nil {
			return fmt.Errorf("%w: reading device_private_key: %v", ErrDeviceMissing, err)
		}
		p.clientID = clientID
		p.privKey = privKey
		return nil
	case WidevineGoWidevine:
		var src gw.DeviceSource
		if p.cfg.WVD != "" {
			f, err := os.Open(p.cfg.WVD)
			if err != nil {
				return fmt.Errorf("%w: opening wvd: %v", ErrDeviceMissing, err)
			}
			defer f.Close()
			src = gw.FromWVD(f)
		} else {
			clientID, err := os.ReadFile(filepath.Join(p.cfg.DeviceDir, "device_client_id_blob"))
			if err != nil {
				return fmt.Errorf("%w: reading device_client_id_blob: %v", ErrDeviceMissing, err)
			}
			privKey, err := os.ReadFile(filepath.Join(p.cfg.DeviceDir, "device_private_key"))
			if err != nil {
				return fmt.Errorf("%w: reading device_private_key: %v", ErrDeviceMissing, err)
			}
			src = gw.FromRaw(clientID, privKey)
		}
		dev, err := gw.NewDevice(src)
		if err != nil {
			return fmt.Errorf("drm: creating gowidevine device: %w", err)
		}
		p.gwDevice = dev
		return nil
	default:
		return fmt.Errorf("%w: unknown widevine backend %q", ErrUnsupported, p.backend)
	}
}

// GetKeys licenses the requested KIDs and returns KID -> CEK.
func (p *WidevineProvider) GetKeys(ctx context.Context, req KeyRequest) (map[KID]CEK, error) {
	headers := headersFromMap(req.Headers, p.cfg.Headers)
	switch p.backend {
	case WidevineDiana:
		return p.keysDiana(ctx, req, headers)
	case WidevineGoWidevine:
		return p.keysGoWidevine(ctx, req, headers)
	default:
		return nil, fmt.Errorf("%w: unknown widevine backend %q", ErrUnsupported, p.backend)
	}
}

// keysDiana licenses via the diana CDM: build PSSH from KID + content ID,
// encode a signed license request, POST it, decode the response keys.
func (p *WidevineProvider) keysDiana(ctx context.Context, req KeyRequest, headers http.Header) (map[KID]CEK, error) {
	pssh := dianaWV.PsshData{ContentId: req.ContentID}
	if len(pssh.ContentId) == 0 && len(req.PSSH) > 0 {
		if d, err := DecodeWidevinePSSH(req.PSSH); err == nil {
			pssh.ContentId = d.ContentID
		}
	}
	for _, kid := range req.KIDs {
		pssh.KeyIds = append(pssh.KeyIds, kid[:])
	}
	if len(pssh.KeyIds) == 0 {
		return nil, ErrKIDNotFound
	}

	reqData, err := pssh.EncodeLicenseRequest(p.clientID)
	if err != nil {
		return nil, fmt.Errorf("drm: encode widevine request: %w", err)
	}
	privKey, err := dianaWV.DecodePrivateKey(p.privKey)
	if err != nil {
		return nil, fmt.Errorf("drm: decode widevine private key: %w", err)
	}
	signed, err := dianaWV.EncodeSignedMessage(reqData, privKey)
	if err != nil {
		return nil, fmt.Errorf("drm: sign widevine request: %w", err)
	}

	resp, err := p.client.post(ctx, req.LicenseURL, httpHeaders(headers), signed)
	if err != nil {
		return nil, err
	}
	keys, err := dianaWV.DecodeLicenseResponse(resp, reqData, privKey)
	if err != nil {
		return nil, fmt.Errorf("drm: decode widevine license: %w", err)
	}

	out := make(map[KID]CEK)
	for _, kid := range req.KIDs {
		key, err := dianaWV.GetKey(keys, kid[:])
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCEKNotFound, err)
		}
		var zero [16]byte
		if len(key) == 16 && equalBytes(key, zero[:]) {
			return nil, errors.New("drm: zero key received")
		}
		out[kid] = append(CEK(nil), key...)
	}
	return out, nil
}

// keysGoWidevine licenses via gowidevine's session-based CDM.
func (p *WidevineProvider) keysGoWidevine(ctx context.Context, req KeyRequest, headers http.Header) (map[KID]CEK, error) {
	if p.gwDevice == nil {
		return nil, ErrDeviceMissing
	}
	pssh, err := gw.NewPSSH(p.widevinePSSHBox(req))
	if err != nil {
		return nil, fmt.Errorf("drm: parse widevine pssh: %w", err)
	}

	cdm := gw.NewCDM(p.gwDevice)
	var cert *widevinepb.DrmCertificate
	if p.cfg.Privacy {
		cert, err = p.serviceCert(ctx)
		if err != nil {
			return nil, err
		}
	}
	challenge, parseLicense, err := cdm.GetLicenseChallenge(pssh, widevinepb.LicenseType_AUTOMATIC, p.cfg.Privacy, cert)
	if err != nil {
		return nil, fmt.Errorf("drm: get license challenge: %w", err)
	}

	resp, err := p.client.post(ctx, req.LicenseURL, httpHeaders(headers), challenge)
	if err != nil {
		return nil, err
	}
	keys, err := parseLicense(resp)
	if err != nil {
		return nil, fmt.Errorf("drm: parse license: %w", err)
	}

	out := make(map[KID]CEK)
	byID := map[string][]byte{}
	for _, k := range keys {
		byID[string(k.ID)] = k.Key
	}
	for _, kid := range req.KIDs {
		key, ok := byID[string(kid[:])]
		if !ok {
			// gowidevine returns the key regardless; match by KID when available.
			continue
		}
		out[kid] = append(CEK(nil), key...)
	}
	if len(out) == 0 && len(keys) > 0 {
		// Fall back to the single content key if KIDs did not match.
		for _, k := range keys {
			var kid KID
			if len(k.ID) == 16 {
				copy(kid[:], k.ID)
			}
			out[kid] = append(CEK(nil), k.Key...)
			break
		}
	}
	if len(out) == 0 {
		return nil, ErrCEKNotFound
	}
	return out, nil
}

// widevinePSSHBox builds a full PSSH box (system ID + payload) for gowidevine
// from the request, preferring the raw box when provided.
func (p *WidevineProvider) widevinePSSHBox(req KeyRequest) []byte {
	if len(req.PSSH) > 0 {
		// req.PSSH carries the box payload; wrap it in a pssh box header.
		return buildPSSHBox(widevineSystemID, req.PSSH)
	}
	return nil
}

// serviceCert fetches the Widevine service certificate for privacy mode.
func (p *WidevineProvider) serviceCert(ctx context.Context) (*widevinepb.DrmCertificate, error) {
	if p.cfg.ServiceCertURL == "" {
		return nil, errors.New("drm: service cert url required in privacy mode")
	}
	certBytes, err := p.client.post(ctx, p.cfg.ServiceCertURL, nil, gw.ServiceCertificateRequest)
	if err != nil {
		return nil, fmt.Errorf("drm: fetch service cert: %w", err)
	}
	cert, err := gw.ParseServiceCert(certBytes)
	if err != nil {
		return nil, fmt.Errorf("drm: parse service cert: %w", err)
	}
	return cert, nil
}
