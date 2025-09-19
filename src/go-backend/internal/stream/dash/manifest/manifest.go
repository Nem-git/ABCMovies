package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/zencoder/go-dash/mpd"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Get(url string, content string) (string, error) {
	content, err := modifyWithGoDash(url, content)
	if err != nil {
		return "", fmt.Errorf("couldn't modify dash manifest using go-dash: %w", err)
	}

	content, err = modifyWithXPath(content)
	if err != nil {
		return "", fmt.Errorf("couldn't modify dash manifest using xpath: %w", err)
	}

	content = modifyWithRegex(content)

	return content, nil
}

func modifyWithGoDash(url string, content string) (string, error) {
	manifest, err := mpd.ReadFromString(content)
	if err != nil {
		return "", fmt.Errorf("parsing dash manifest from string failed: %w", err)
	}

	baseUrl, err := utils.JoinUrls(url, manifest.BaseURL) // Joins mpd and baseurl urls
	if err != nil {
		return "", fmt.Errorf("couldn't join manifest url with it's base url: %w", err)
	}

	for _, period := range manifest.Periods {
		periodUrl, err := utils.JoinUrls(baseUrl, period.BaseURL)
		if err != nil {
			return "", fmt.Errorf("failed to join manifest and period urls: %w", err)
		}

		for _, adaptationSet := range period.AdaptationSets {
			err := modifySegmentTemplate(adaptationSet.SegmentTemplate, periodUrl)
			if err != nil {
				return "", fmt.Errorf("couldn't modify segment template in adaptation set urls: %w", err)
			}

			for _, representation := range adaptationSet.Representations {
				err := modifySegmentTemplate(representation.SegmentTemplate, periodUrl)
				if err != nil {
					return "", fmt.Errorf("couldn't modify segment template in representation urls: %w", err)
				}
			}
		}
	}

	content, err = manifest.WriteToString()
	if err != nil {
		return "", fmt.Errorf("parsing dash manifest to string failed: %w", err)
	}

	return content, nil
}

func modifyWithXPath(content string) (string, error) {

	doc, err := xmlquery.Parse(strings.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("couldn't parse xml using xpath")
	}

	removeNodes(doc, "//BaseURL")
	removeNodes(doc, "//ContentProtection")
	removeNodes(doc, "//EventStream")

	return doc.OutputXMLWithOptions(), nil
}

func removeNodes(doc *xmlquery.Node, exp string) {
	nodes := xmlquery.Find(doc, exp)

	for _, node := range nodes {
		xmlquery.RemoveFromTree(node)
	}
}

func modifyWithRegex(content string) string {

	for _, re := range []string{config.RE_DASH_MANIFEST_CENC, config.RE_DASH_MANIFEST_PLAYREADY} {
		content = removeWithRegex(content, re)
	}

	return content
}

func removeWithRegex(content string, re string) string {
	r := regexp.MustCompilePOSIX(re)
	return r.ReplaceAllString(content, "")
}

func modifySegmentTemplate(segmentTemplate *mpd.SegmentTemplate, periodUrl string) error {

	var err error // Because Initialization and Media are pointers

	if segmentTemplate != nil {
		if segmentTemplate.Initialization != nil {
			id := utils.GetUniqueId()

			*segmentTemplate.Initialization, err = modifySegmentUrl(*segmentTemplate.Initialization, periodUrl, config.DASH_INIT_URL_PREFIX, id)
			if err != nil {
				return fmt.Errorf("couldn't modify segment initialization url: %w", err)
			}

			if segmentTemplate.Media != nil {
				*segmentTemplate.Media, err = modifySegmentUrl(*segmentTemplate.Media, periodUrl, config.DASH_MEDIA_URL_PREFIX, id)
				if err != nil {
					return fmt.Errorf("couldn't modify segment initialization url: %w", err)
				}
			} else {
				return fmt.Errorf("init found but no media segment")
			}
		} else if segmentTemplate.Media != nil {
			*segmentTemplate.Media, err = modifySegmentUrl(*segmentTemplate.Media, periodUrl, config.DASH_MEDIA_URL_PREFIX)
			if err != nil {
				return fmt.Errorf("couldn't modify segment initialization url: %w", err)
			}
		} else {
			return fmt.Errorf("no init or media segment found")
		}
	}

	return nil
}

func modifySegmentUrl(segmentUrl string, periodUrl string, prefix string, id ...string) (string, error) {

	strId := ""
	if id != nil {
		strId = id[0]
	}

	mergedUrl, err := utils.JoinUrls(periodUrl, segmentUrl)
	if err != nil {
		return "", fmt.Errorf("couldn't join segment and period url: %w", err)
	}

	formattedUrl, err := utils.CreateCustomUrl(mergedUrl, prefix, strId)
	if err != nil {
		return "", fmt.Errorf("couldn't create the custom url: %w", err)
	}

	return formattedUrl, nil
}
