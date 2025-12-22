package utils

import (
	"errors"

	"github.com/pariz/gountries"
)

func CountryNameToEnglish(name string) (string, error) {
	c, err := gountries.New().FindCountryByName(name) // https://en.wikipedia.org/wiki/List_of_ISO_3166_country_codes
	if err != nil {
		return "", errors.New("couldn't change country name to english")
	}

	return c.Name.Common, nil
}

func CountryNameToTag(name string) (string, error) {
	c, err := gountries.New().FindCountryByName(name) // https://en.wikipedia.org/wiki/List_of_ISO_3166_country_codes
	if err != nil {
		return "", errors.New("couldn't change country name to tag")
	}

	return c.Alpha3, nil
}
