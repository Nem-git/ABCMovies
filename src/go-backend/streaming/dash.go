package streaming

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/zencoder/go-dash/mpd"

	"github.com/nem-git/abcmovies/config"
	"github.com/nem-git/abcmovies/utils"
)

type Dash struct{}

func (d *Dash) GetModifiedManifest(url string, content string) (string, error) {
	content, err := d.modifyWithGoDash(url, content)
	if err != nil {
		return "", fmt.Errorf("couldn't modify dash manifest using go-dash: %w", err)
	}

	content, err = d.modifyWithXPath(content)
	if err != nil {
		return "", fmt.Errorf("couldn't modify dash manifest using xpath: %w", err)
	}

	content = d.modifyWithRegex(content)

	return content, nil
}

func (d *Dash) modifyWithGoDash(url string, content string) (string, error) {
	manifest, err := mpd.ReadFromString(content)
	if err != nil {
		return "", fmt.Errorf("parsing dash manifest from string failed: %w", err)
	}

	baseUrl, err := utils.JoinUrls(url, manifest.BaseURL) // Joins mpd and baseurl urls

	for _, period := range manifest.Periods {
		periodUrl, err := utils.JoinUrls(baseUrl, period.BaseURL)
		if err != nil {
			return "", fmt.Errorf("failed to join manifest and period urls: %w", err)
		}

		for _, adaptationSet := range period.AdaptationSets {
			err := d.modifySegmentTemplate(adaptationSet.SegmentTemplate, periodUrl)
			if err != nil {
				return "", fmt.Errorf("couldn't modify segment template in adaptation set urls: %w", err)
			}

			for _, representation := range adaptationSet.Representations {
				err := d.modifySegmentTemplate(representation.SegmentTemplate, periodUrl)
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

func (d *Dash) modifyWithXPath(content string) (string, error) {

	doc, err := xmlquery.Parse(strings.NewReader(content))
	if err != nil {
		return "", fmt.Errorf("couldn't parse xml using xpath")
	}

	d.removeNodes(doc, "//BaseURL")
	d.removeNodes(doc, "//ContentProtection")
	d.removeNodes(doc, "//EventStream")

	return doc.OutputXMLWithOptions(), nil
}

func (d *Dash) removeNodes(doc *xmlquery.Node, exp string) {
	nodes := xmlquery.Find(doc, exp)

	for _, node := range nodes {
		xmlquery.RemoveFromTree(node)
	}
}

func (d *Dash) modifyWithRegex(content string) string {

	for _, re := range []string{config.RE_DASH_MANIFEST_CENC, config.RE_DASH_MANIFEST_PLAYREADY} {
		content = d.removeWithRegex(content, re)
	}

	return content
}

func (d *Dash) removeWithRegex(content string, re string) string {
	r := regexp.MustCompilePOSIX(re)
	return r.ReplaceAllString(content, "")
}

func (d *Dash) modifySegmentTemplate(segmentTemplate *mpd.SegmentTemplate, periodUrl string) error {

	var err error // Because Initialization and Media are pointers

	if segmentTemplate != nil {
		if segmentTemplate.Initialization != nil {
			id := utils.GetUniqueId()

			*segmentTemplate.Initialization, err = d.modifySegmentUrl(*segmentTemplate.Initialization, periodUrl, config.DASH_INIT_URL_PREFIX, id)
			if err != nil {
				return fmt.Errorf("couldn't modify segment initialization url: %w", err)
			}

			if segmentTemplate.Media != nil {
				*segmentTemplate.Media, err = d.modifySegmentUrl(*segmentTemplate.Media, periodUrl, config.DASH_MEDIA_URL_PREFIX, id)
				if err != nil {
					return fmt.Errorf("couldn't modify segment initialization url: %w", err)
				}
			} else {
				return fmt.Errorf("init found but no media segment")
			}
		} else if segmentTemplate.Media != nil {
			*segmentTemplate.Media, err = d.modifySegmentUrl(*segmentTemplate.Media, periodUrl, config.DASH_MEDIA_URL_PREFIX)
			if err != nil {
				return fmt.Errorf("couldn't modify segment initialization url: %w", err)
			}
		} else {
			return fmt.Errorf("no init or media segment found")
		}
	}

	return nil
}

func (d *Dash) modifySegmentUrl(segmentUrl string, periodUrl string, prefix string, id ...string) (string, error) {

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
