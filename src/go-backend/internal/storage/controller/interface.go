package controller

import "github.com/google/uuid"

// Database connection
type LoginDetails struct {
	Addr     string
	Password string
	DB       int
	Protocol int
}

type BaseController interface {
	// Connection
	SetupDatabase(LoginDetails) error

	// Dash Modified Manifest
	GetDashManifest(uuid.UUID) (string, error)

	// Widevine Decryption Keys
	GetWidevineKeys(uuid.UUID) ([]string, error)
	// Widevine PSSH data
	GetWidevinePssh(uuid.UUID) ([]byte, error)
	// Widevine Init Segment
	GetWidevineInit(uuid.UUID) ([]byte, error)
}
