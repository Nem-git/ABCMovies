package utils

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/nem-git/abcmovies/internal/config"
)

func OpenCdmFile(filePath string) (*os.File, error) {
	abs, err := filepath.Abs(config.CDM_PATH)
	if err != nil {
		return nil, errors.New("couldn't change cdm path to absolute")
	}
	path := filepath.Join(abs, filePath)
	return os.Open(path)
}

func GetFile(filePath string) ([]byte, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return data, err
}

func GetCdmFile(filePath string) ([]byte, error) {
	abs, err := filepath.Abs(config.CDM_PATH)
	if err != nil {
		return nil, errors.New("couldn't change cdm path to absolute")
	}
	path := filepath.Join(abs, filePath)
	return GetFile(path)
}
