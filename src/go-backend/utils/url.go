package utils

import (
	"fmt"
	"net/url"
	"strings"
)

func JoinUrls(urls ...string) (string, error) {
	joined := ""

	if len(urls) == 0 {
		return "", fmt.Errorf("no urls given to join")
	}

	for _, u := range urls {
		joinedUrlPath, err := url.Parse(joined)
		if err != nil {
			return "", fmt.Errorf("couldn't join urls provided: %w", err)
		}
		urlPath, err := url.Parse(u)
		if err != nil {
			return "", fmt.Errorf("couldn't join urls provided: %w", err)
		}
		joined = joinedUrlPath.ResolveReference(urlPath).String()
	}

	return joined, nil
}

func CreateCustomUrl(fullUrl string, prefix string, id ...string) (string, error) {

	strId := ""
	if id != nil {
		strId = id[0]
	}

	uPath, err := url.Parse(fullUrl)
	if err != nil {
		return "", fmt.Errorf("could't parse url: %w", err)
	}

	newUrl := strings.Join([]string{uPath.Scheme, uPath.Host}, "/") + uPath.Path // Because it contains the first /

	if len(uPath.RawQuery) > 0 {
		newUrl += "?" + uPath.RawQuery
	}

	if len(uPath.RawFragment) > 0 {
		newUrl += "#" + uPath.RawFragment
	}

	// Until now, the url looks like this: http/domain.com/thing?wow=a#3
	// it does not contain the prefix nor does it contain the id (if it exists)

	newUrl = strings.Join([]string{prefix, strId, newUrl}, "/")

	return newUrl, nil
}
