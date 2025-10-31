package manifest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/antchfx/xmlquery"
	"github.com/zencoder/go-dash/mpd"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/storage/cache/connector"
	dashController "github.com/nem-git/abcmovies/internal/storage/cache/controller/dash"
	dashRepo "github.com/nem-git/abcmovies/internal/storage/cache/repository/dash"
	"github.com/nem-git/abcmovies/internal/utils"
)

func Get(url string, content string, dbID string) (string, error) {

	man, err := GetUsingDB(dbID)
	if err == nil {
		return man, nil
	}

	content, err = modifyWithGoDash(url, content)
	if err != nil {
		return "", fmt.Errorf("couldn't modify dash manifest using go-dash: %w", err)
	}

	content, err = modifyWithXPath(content)
	if err != nil {
		return "", fmt.Errorf("couldn't modify dash manifest using xpath: %w", err)
	}

	content = modifyWithRegex(content)

	if err := SaveUsingDB(dbID, content); err != nil {
		return "", err
	}

	return content, nil
}

func modifyWithGoDash(url string, content string) (string, error) {
	manifest, err := mpd.ReadFromString(content)
	if err != nil {
		return "", fmt.Errorf("parsing dash manifest from string failed: %w", err)
	}

	baseUrl, err := utils.JoinURLs(url, manifest.BaseURL) // Joins mpd and baseurl urls
	if err != nil {
		return "", fmt.Errorf("couldn't join manifest url with it's base url: %w", err)
	}

	for _, period := range manifest.Periods {
		periodUrl, err := utils.JoinURLs(baseUrl, period.BaseURL)
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
			id := utils.GetUniqueID()

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

	strID := ""
	if id != nil {
		strID = id[0]
	}

	mergedUrl, err := utils.JoinURLs(periodUrl, segmentUrl)
	if err != nil {
		return "", fmt.Errorf("couldn't join segment and period url: %w", err)
	}

	formattedUrl, err := utils.CreateCustomURL(mergedUrl, prefix, strID)
	if err != nil {
		return "", fmt.Errorf("couldn't create the custom url: %w", err)
	}

	return formattedUrl, nil
}

func GetUsingDB(dbID string) (string, error) {
	conn := connector.NewRedisConnector(connector.ConnectionDetails{
		Address:  config.TEMP_REDIS_ADDRESS,
		User:     config.TEMP_REDIS_USER,
		Password: config.TEMP_REDIS_PASSWORD,
		DB:       config.TEMP_REDIS_DB,
	})
	repo := dashRepo.NewManifestRepository(conn)
	controller := dashController.NewManifestController(repo)

	manifest, err := controller.ReadSingle(dbID)
	if err != nil {
		return "", err
	}

	return manifest, nil
}

func SaveUsingDB(dbID string, content string) error {
	conn := connector.NewRedisConnector(connector.ConnectionDetails{
		Address:  config.TEMP_REDIS_ADDRESS,
		User:     config.TEMP_REDIS_USER,
		Password: config.TEMP_REDIS_PASSWORD,
		DB:       config.TEMP_REDIS_DB,
	})
	repo := dashRepo.NewManifestRepository(conn)
	controller := dashController.NewManifestController(repo)

	if err := controller.Create(dbID, content); err != nil {
		return err
	}

	return nil
}
