package drm

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/Eyevinn/mp4ff/bits"
	"github.com/Eyevinn/mp4ff/mp4"
	"github.com/antchfx/xmlquery"
	widevine "github.com/iyear/gowidevine"
	"github.com/iyear/gowidevine/widevinepb"
	"github.com/zencoder/go-dash/mpd"

	"github.com/nem-git/abcmovies/config"
	"github.com/nem-git/abcmovies/utils"
)

type Widevine struct{}

type DashPlaceHolder struct {
	number           int
	time             int
	bandwidth        int
	representationId string
}

func (w *Widevine) GetDecryptedSegment(initStr string, segmentStr string, keys []string, isInit bool) (string, error) {

	if initStr == "" {
		return "", fmt.Errorf("no init segment provided")
	}
	if segmentStr == "" && !isInit {
		return "", fmt.Errorf("no media segment provided")
	}

	initByte, err := base64.RawStdEncoding.DecodeString(initStr)
	if err != nil {
		return "", fmt.Errorf("couldn't decode init: %w", err)
	}

	sr := bits.NewFixedSliceReader(initByte)

	inputFile, err := mp4.DecodeFileSR(sr)
	if err != nil {
		return "", fmt.Errorf("couldn't parse init: %w", err)
	}

	init := inputFile.Init
	if inputFile.Init == nil {
		return "", fmt.Errorf("couldn't transform init to mp4.Init")
	}

	di, err := mp4.DecryptInit(init)
	if err != nil {
		return "", fmt.Errorf("couldn't clean init: %w", err)
	}

	if isInit {
		sw := bits.NewFixedSliceWriter(int(init.Size()))
		encodedInit := base64.RawStdEncoding.EncodeToString(sw.Bytes())
		return encodedInit, nil
	} else {

		segmentByte, err := base64.RawStdEncoding.DecodeString(segmentStr)
		if err != nil {
			return "", fmt.Errorf("couldn't decode segment: %w", err)
		}

		sr := bits.NewFixedSliceReader(segmentByte)

		segmentFile, err := mp4.DecodeFileSR(sr)
		if err != nil {
			return "", fmt.Errorf("couldn't parse init: %w", err)
		}

		segments := segmentFile.Segments

		file := mp4.NewFile()

		for _, segment := range segments {
			for _, k := range keys {
				key, err := mp4.UnpackKey(k)
				if err != nil {
					mp4.DecryptSegment(segment, di, key)
					file.AddMediaSegment(segment)
				}
			}
		}

		sw := bits.NewFixedSliceWriter(int(file.Size()))
		encodedMedia := base64.RawStdEncoding.EncodeToString(sw.Bytes())
		return encodedMedia, nil
	}
}

func (w *Widevine) mergeSegments(init *mp4.File, segment *mp4.File) (*mp4.File, error) {

	// mp4.MediaSegment
	// mp4.InitSegment
	// mp4.DecryptInit()

	return nil, nil

}

func (w *Widevine) init(encodedPssh string) (*widevine.Device, *widevine.PSSH, error) {
	var device *widevine.Device

	psshBytes, err := base64.RawStdEncoding.DecodeString(encodedPssh)
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

func (w *Widevine) GetKeys(psshData string, licenseUrl string, headers map[string]string) ([]string, error) {

	device, pssh, err := w.init(psshData)
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
		cert, err := w.getServiceCert(licenseUrl)
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

func (w *Widevine) getServiceCert(licenseUrl string) (*widevinepb.DrmCertificate, error) {

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

func (w *Widevine) GetPssh(url string, headers map[string]string, segHeaders map[string]string) (string, error) {

	// Send request to get Dash Manifest
	body, err := utils.Get(url, nil, headers)
	if err != nil {
		return "", fmt.Errorf("couldn't retrieve body: %w", err)
	}

	pssh, err := w.psshWithManifest(body)
	if err == nil {
		return pssh, nil
	}

	pssh, err = w.psshWithSegment(url, body, segHeaders)
	if err != nil {
		return "", fmt.Errorf("couldn't retrieve pssh")
	}

	return pssh, nil
}

func (w *Widevine) psshWithSegment(url string, b io.ReadCloser, segHeaders map[string]string) (string, error) {

	content, err := io.ReadAll(b)
	if err != nil {
		return "", fmt.Errorf("couldn't read request body: %w", err)
	}

	url, err = w.readWithGoDash(url, string(content))
	if err != nil {
		return "", fmt.Errorf("couldn't get segment url using dash manifest: %w", err)
	}

	// Send request to get Segment content
	body, err := utils.Get(url, nil, segHeaders)
	if err != nil {
		return "", fmt.Errorf("couldn't make segment request: %w", err)
	}

	dataBytes, err := io.ReadAll(body)
	if err != nil {
		return "", fmt.Errorf("couldn't parse segment body: %w", err)
	}

	sr := bits.NewFixedSliceReader(dataBytes)

	parsed, err := mp4.DecodeFileSR(sr)
	if err != nil || parsed == nil {
		return "", fmt.Errorf("couldn't parse segment body: %w", err)
	}

	if parsed.Init != nil {

		decryptInfo, err := mp4.DecryptInit(parsed.Init)
		if err == nil {
			if decryptInfo.Psshs != nil {
				for _, pssh := range decryptInfo.Psshs {
					buf := new(bytes.Buffer)

					err = pssh.Encode(buf)
					if err != nil {
						continue
					}

					return base64.RawStdEncoding.EncodeToString(buf.Bytes()), nil
				}
			}
		}
	}

	if parsed.Segments != nil {
		if parsed.Moov == nil {
			return "", fmt.Errorf("couldn't find moov atom in mp4")
		}
		if parsed.Moov.Psshs == nil {
			return "", fmt.Errorf("couldn't find pssh atom in mp4")
		}

		for _, pssh := range parsed.Moov.Psshs {
			if pssh.SystemID.Equal(mp4.UUID(mp4.UUIDWidevine)) {

				buf := new(bytes.Buffer)

				err = pssh.Encode(buf)
				if err != nil {
					continue
				}

				return base64.RawStdEncoding.EncodeToString(buf.Bytes()), nil
			}
		}
	}

	return "", fmt.Errorf("no init or segment found")
}

func (d *Widevine) readWithGoDash(url string, content string) (string, error) {

	manifest, err := mpd.ReadFromString(content)
	if err != nil {
		return "", fmt.Errorf("parsing dash manifest from string failed: %w", err)
	}

	baseUrl, err := utils.JoinUrls(url, manifest.BaseURL) // Joins mpd and baseurl urls
	if err != nil {
		return "", fmt.Errorf("couldn't join urls: %w", err)
	}

	if manifest.Periods == nil {
		return "", fmt.Errorf("no periods found in dash manifest")
	}

	for _, period := range manifest.Periods {

		periodUrl, err := utils.JoinUrls(baseUrl, period.BaseURL)
		if err != nil {
			return "", fmt.Errorf("failed to join manifest and period urls: %w", err)
		}

		if period.AdaptationSets == nil {
			continue
		}

		for _, adaptationSet := range period.AdaptationSets {

			// At least I think representations cannot be in segment templates...
			if adaptationSet.Representations == nil {
				continue
			}

			if adaptationSet.SegmentTemplate != nil {

				// Don't rename it because I make sure to not mix values between representations
				urlWithPlaceHolders, segmentTemplaceDashPlaceHolders, err := d.segmentPathWithGoDash(adaptationSet.SegmentTemplate, periodUrl)
				if err != nil {
					continue
				}

				for _, representation := range adaptationSet.Representations {
					dashPlaceHolder := d.representationWithGoDash(representation, segmentTemplaceDashPlaceHolders)
					return d.replaceDashUrlPlaceholders(urlWithPlaceHolders, dashPlaceHolder), nil
				}

			} else {
				for _, representation := range adaptationSet.Representations {

					// That would be stupid..
					if representation.SegmentTemplate == nil {
						continue
					}

					urlWithPlaceHolders, dashPlaceHolder, err := d.segmentPathWithGoDash(representation.SegmentTemplate, periodUrl)
					if err != nil {
						continue
					}

					dashPlaceHolder = d.representationWithGoDash(representation, dashPlaceHolder)
					return d.replaceDashUrlPlaceholders(urlWithPlaceHolders, dashPlaceHolder), nil
				}
			}

		}
	}

	return url, nil
}

func (w *Widevine) segmentPathWithGoDash(segmentTemplate *mpd.SegmentTemplate, url string) (string, DashPlaceHolder, error) {

	if segmentTemplate == nil {
		return "", DashPlaceHolder{}, fmt.Errorf("segment template is nil")
	}

	segmentUrl := ""

	if segmentTemplate.Initialization != nil {
		segmentUrl = *segmentTemplate.Initialization
	} else if segmentTemplate.Media != nil {
		segmentUrl = *segmentTemplate.Media
	} else {
		return "", DashPlaceHolder{}, fmt.Errorf("no init or media url found in segment template")
	}

	joinedUrl, err := utils.JoinUrls(url, segmentUrl)
	if err != nil {
		return "", DashPlaceHolder{}, fmt.Errorf("couldn't join period and segment url: %w", err)
	}

	var dashPlaceHolder DashPlaceHolder

	if segmentTemplate.StartNumber != nil {
		dashPlaceHolder.number = int(*segmentTemplate.StartNumber)
	}
	if segmentTemplate.PresentationTimeOffset != nil {
		dashPlaceHolder.time = int(*segmentTemplate.PresentationTimeOffset)
	}

	return joinedUrl, dashPlaceHolder, nil
}

func (w *Widevine) representationWithGoDash(representation *mpd.Representation, dashPlaceHolder DashPlaceHolder) DashPlaceHolder {
	dashPlaceHolder.bandwidth = int(*representation.Bandwidth)
	dashPlaceHolder.representationId = *representation.ID

	return dashPlaceHolder
}

func (w *Widevine) replaceDashUrlPlaceholders(url string, dashPlaceHolder DashPlaceHolder) string {

	url = strings.ReplaceAll(url, "$Number$", strconv.Itoa(dashPlaceHolder.number))
	url = strings.ReplaceAll(url, "$Time$", strconv.Itoa(dashPlaceHolder.time))
	url = strings.ReplaceAll(url, "$Bandwidth$", strconv.Itoa(dashPlaceHolder.bandwidth))
	url = strings.ReplaceAll(url, "$RepresentationID$", dashPlaceHolder.representationId)

	return url
}

func (w *Widevine) psshWithManifest(body io.ReadCloser) (string, error) {

	doc, err := xmlquery.Parse(body)
	if err != nil {
		return "", fmt.Errorf("couldn't parse xml using xpath")
	}

	nodes := xmlquery.Find(doc, "//ContentProtection")

	for _, node := range nodes {
		for range node.Attr {
			scheme := strings.ToUpper(node.SelectAttr("schemeIdUri"))
			switch scheme {

			// Directly in ContentProtection
			case strings.Join([]string{"URN", "UUID", config.WIDEVINE_UUID}, ":"):
				pssh, err := w.psshContentProtection(node)
				if err == nil {
					return pssh, err
				}

			// Default KID
			case strings.Join([]string{"URN", config.CENC_SCHEME_ID}, ":"):
				pssh, err := w.psshDefaultKID(node)
				if err == nil {
					return pssh, err
				}
			}
		}
	}

	return "", fmt.Errorf("couldn't find pssh from contentprotection or default kid")

}

func (w *Widevine) psshDefaultKID(node *xmlquery.Node) (string, error) {
	defaultKID := node.SelectAttr("cenc:default_KID")

	if defaultKID == "" {
		return "", fmt.Errorf("couln't find kid attribute in contentprotection")
	}

	defaultKID = strings.ReplaceAll(defaultKID, "-", "")

	decodedPssh := strings.Join([]string{config.WIDEVINE_PSSH_PART_1, defaultKID, config.WIDEVINE_PSSH_PART_3}, "")

	hexPssh, err := hex.DecodeString(decodedPssh)
	if err != nil {
		return "", fmt.Errorf("couldn't convert pssh from string to hex: %w", err)
	}

	return base64.RawStdEncoding.EncodeToString(hexPssh), nil

}

func (w *Widevine) psshContentProtection(node *xmlquery.Node) (string, error) {
	pssh := ""

	currentChild := node.FirstChild
	beenThroughFirstChild := false

	for {
		if currentChild.Data == "pssh" {
			pssh = currentChild.InnerText()
			break
		}

		if currentChild == node.FirstChild && beenThroughFirstChild {
			break
		} else {
			currentChild = currentChild.NextSibling
		}

		if !beenThroughFirstChild {
			beenThroughFirstChild = !beenThroughFirstChild
		}
	}

	if pssh == "" {
		return pssh, fmt.Errorf("couldn't find pssh inside of contentprotection")
	}

	return pssh, nil
}
