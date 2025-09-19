package pssh

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
	"github.com/zencoder/go-dash/mpd"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Get(url string, headers map[string]string, segHeaders map[string]string) (string, error) {

	// Send request to get Dash Manifest
	body, err := utils.Get(url, nil, headers)
	if err != nil {
		return "", fmt.Errorf("couldn't retrieve body: %w", err)
	}

	pssh, err := psshWithManifest(body)
	if err == nil {
		return pssh, nil
	}

	pssh, err = psshWithSegment(url, body, segHeaders)
	if err != nil {
		return "", fmt.Errorf("couldn't retrieve pssh")
	}

	return pssh, nil
}

func psshWithSegment(url string, b io.ReadCloser, segHeaders map[string]string) (string, error) {

	content, err := io.ReadAll(b)
	if err != nil {
		return "", fmt.Errorf("couldn't read request body: %w", err)
	}

	url, err = readWithGoDash(url, string(content))
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

					return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
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

				return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
			}
		}
	}

	return "", fmt.Errorf("no init or segment found")
}

func readWithGoDash(url string, content string) (string, error) {

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
				urlWithPlaceHolders, segmentTemplaceDashPlaceHolders, err := segmentPathWithGoDash(adaptationSet.SegmentTemplate, periodUrl)
				if err != nil {
					continue
				}

				for _, representation := range adaptationSet.Representations {
					dashPlaceHolder := representationWithGoDash(representation, segmentTemplaceDashPlaceHolders)
					return replaceDashUrlPlaceholders(urlWithPlaceHolders, dashPlaceHolder), nil
				}

			} else {
				for _, representation := range adaptationSet.Representations {

					// That would be stupid..
					if representation.SegmentTemplate == nil {
						continue
					}

					urlWithPlaceHolders, dashPlaceHolder, err := segmentPathWithGoDash(representation.SegmentTemplate, periodUrl)
					if err != nil {
						continue
					}

					dashPlaceHolder = representationWithGoDash(representation, dashPlaceHolder)
					return replaceDashUrlPlaceholders(urlWithPlaceHolders, dashPlaceHolder), nil
				}
			}

		}
	}

	return url, nil
}

func psshWithManifest(body io.ReadCloser) (string, error) {

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
				pssh, err := psshContentProtection(node)
				if err == nil {
					return pssh, err
				}

			// Default KID
			case strings.Join([]string{"URN", config.CENC_SCHEME_ID}, ":"):
				pssh, err := psshDefaultKID(node)
				if err == nil {
					return pssh, err
				}
			}
		}
	}

	return "", fmt.Errorf("couldn't find pssh from contentprotection or default kid")

}

func psshDefaultKID(node *xmlquery.Node) (string, error) {
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

	return base64.StdEncoding.EncodeToString(hexPssh), nil

}

func psshContentProtection(node *xmlquery.Node) (string, error) {
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

type DashPlaceHolder struct {
	number           int
	time             int
	bandwidth        int
	representationId string
}

func segmentPathWithGoDash(segmentTemplate *mpd.SegmentTemplate, url string) (string, DashPlaceHolder, error) {

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

func representationWithGoDash(representation *mpd.Representation, dashPlaceHolder DashPlaceHolder) DashPlaceHolder {
	dashPlaceHolder.bandwidth = int(*representation.Bandwidth)
	dashPlaceHolder.representationId = *representation.ID

	return dashPlaceHolder
}

func replaceDashUrlPlaceholders(url string, dashPlaceHolder DashPlaceHolder) string {

	url = strings.ReplaceAll(url, "$Number$", strconv.Itoa(dashPlaceHolder.number))
	url = strings.ReplaceAll(url, "$Time$", strconv.Itoa(dashPlaceHolder.time))
	url = strings.ReplaceAll(url, "$Bandwidth$", strconv.Itoa(dashPlaceHolder.bandwidth))
	url = strings.ReplaceAll(url, "$RepresentationID$", dashPlaceHolder.representationId)

	return url
}
