package utils

import (
	"github.com/google/uuid"
)

func GetUniqueId() string {
	return uuid.New().String()
}
