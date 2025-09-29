package utils

import (
	"errors"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func LanguageNameToEnglish(name string) (string, error) {
	t, err := language.Parse(name)
	if err != nil {
		return "", errors.New("couldn't parse language name to english")
	}

	caser := cases.Title(t)
	return caser.String(t.String()), nil
}

func LanguageNameToNative(name string) (string, error) {
	t, err := language.Parse(name)
	if err != nil {
		return "", errors.New("couldn't change language name to native")
	}

	caser := cases.Title(t)
	return caser.String(display.Self.Name(t)), nil
}

func LanguageNameToTag(name string) (string, error) {
	t, err := language.Parse(name)
	if err != nil {
		return "", errors.New("couldn't change language name to tag")
	}

	return t.String(), nil
}
