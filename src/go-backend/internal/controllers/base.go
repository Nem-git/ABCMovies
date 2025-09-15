package controllers

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
	SetupDatabase(ld LoginDetails) error

	// Dash Modified Manifest
	GetDashManifest(id uuid.UUID) (string, error)

	// Widevine Decryption Keys
	GetWidevineKeys(id uuid.UUID) ([]string, error)
	// Widevine PSSH data
	GetWidevinePssh(id uuid.UUID) ([]byte, error)
	// Widevine Init Segment
	GetWidevineInit(id uuid.UUID) ([]byte, error)
}
