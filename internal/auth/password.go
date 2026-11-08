// Package auth holds password hashing helpers (argon2id).
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	// timeCost is the number of argon2 iterations.
	timeCost = 2
	// memoryCost is the memory usage in kibibytes (32 MiB).
	memoryCost = 32 * 1024
	// parallelism is argon2's lane count.
	parallelism = 1
	// saltLength is the length of a random salt in bytes.
	saltLength = 16
	// keyLength is the output hash length in bytes.
	keyLength = 32
)

// HashPassword hashes password with argon2id and returns a self-contained
// encoded string: "$argon2id$v=19$m=<mem>,t=<time>,p=<par>$<salt>$<hash>".
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	digest := argon2.IDKey([]byte(password), salt, timeCost, memoryCost, parallelism, keyLength)
	header := fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$", memoryCost, timeCost, parallelism)
	return header + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(digest), nil
}

// VerifyPassword reports whether password matches the encoded argon2id hash.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false, errors.New("invalid encoded hash: expected 6 segments")
	}

	var memory, time uint32
	var parallel uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &parallel); err != nil {
		return false, fmt.Errorf("invalid encoded hash params: %w", err)
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("invalid salt encoding: %w", err)
	}
	digest, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("invalid hash encoding: %w", err)
	}

	computed := argon2.IDKey([]byte(password), salt, time, memory, parallel, uint32(len(digest)))
	return subtle.ConstantTimeCompare(digest, computed) == 1, nil
}