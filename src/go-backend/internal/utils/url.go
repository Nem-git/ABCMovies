package utils

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/nem-git/abcmovies/internal/config"
	"github.com/nem-git/abcmovies/internal/errs"
)

func JoinURLs(urls ...string) (string, error) {
	joined := ""

	if len(urls) == 0 {
		return "", errors.New("no urls given to join")
	}

	for _, u := range urls {
		joinedURLPath, err := url.Parse(joined)
		if err != nil {
			return "", errors.New("couldn't join urls provided")
		}
		urlPath, err := url.Parse(u)
		if err != nil {
			return "", errors.New("couldn't join urls provided")
		}
		joined = joinedURLPath.ResolveReference(urlPath).String()
	}

	return joined, nil
}

func CreateCustomURL(fullURL string, prefix string, id ...string) (string, error) {

	strID := ""
	if id != nil {
		strID = id[0]
	}

	uPath, err := url.Parse(fullURL)
	if err != nil {
		return "", errs.ErrInvalidURL
	}

	newURL := strings.Join([]string{uPath.Scheme, uPath.Host}, "/") + uPath.Path // Because it contains the first /

	if len(uPath.RawQuery) > 0 {
		newURL += "?" + uPath.RawQuery
	}

	if len(uPath.RawFragment) > 0 {
		newURL += "#" + uPath.RawFragment
	}

	// Until now, the url looks like this: http/domain.com/thing?wow=a#3
	// it does not contain the prefix nor does it contain the id (if it exists)

	newURL = strings.Join([]string{prefix, strID, newURL}, "/")

	return newURL, nil
}

func CreateStreamURL(serviceTag string, showID string, seasonNumber int, episodeNumber int, streamType string) (string, error) {

	backendURL, ok := os.LookupEnv(config.BACKEND_URL_ENV_NAME)
	if !ok {
		return "", errs.ErrEmptyBackendURLEnv
	}

	uPath, err := url.Parse(backendURL)
	if err != nil {
		return "", errs.ErrInvalidURL
	}

	fileName := config.STREAM_TYPE_TO_FILE_NAME[streamType]

	uPath = uPath.JoinPath(
		"service",
		serviceTag,
		showID,
		strconv.Itoa(seasonNumber),
		strconv.Itoa(episodeNumber),
		streamType, fileName,
	)

	return uPath.String(), nil
}
