package utils

import (
	"os"
	"path/filepath"

	"github.com/nem-git/abcmovies/config"
)

func OpenCdmFile(filePath string) (*os.File, error) {
	path := filepath.Join(config.CDM_PATH, filePath)
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
	path := filepath.Join(config.CDM_PATH, filePath)
	return GetFile(path)
}
