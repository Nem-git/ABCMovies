package config

import "os"

const (
	FRONTEND_URL_ENV_NAME string = "FRONTEND_URL"
	BACKEND_URL_ENV_NAME  string = "BACKEND_URL"

	// Database
	DATABASE_ADDRESS_ENV_NAME  string = "DB_ADDR"
	DATABASE_PASSWORD_ENV_NAME string = "DB_PW"
	DATABASE_ID_ENV_NAME       string = "DB_ID"
)

var (
	DB_ADDRESS  = os.Getenv(DATABASE_ADDRESS_ENV_NAME)
	DB_PASSWORD = os.Getenv(DATABASE_PASSWORD_ENV_NAME)
	DB_ID       = os.Getenv(DATABASE_ID_ENV_NAME)
)
