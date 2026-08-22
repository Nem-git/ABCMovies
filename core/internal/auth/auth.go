// Package auth provides user authentication and key derivation for ABCMovies.
//
// The server performs all cryptographic operations: Argon2id key derivation,
// DEK wrapping/unwrapping, and recovery key generation. The client sends
// passwords; the server never exposes raw keys or wrapped material.
package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Pinned Argon2id parameters (TECHNICAL-DECISIONS §1.11).
const (
	argon2Memory  = 19 * 1024 // 19 MiB in KiB
	argon2Time    = 2
	argon2Threads = 1
	argon2KeyLen  = 32
	saltLen       = 16
	dekLen        = 32
)

// base32 alphabet (RFC 4648, no padding).
const base32Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"

// SignUpResult contains the output of a successful sign-up.
type SignUpResult struct {
	UserID      string
	RecoveryKey string // 128-bit base32, shown once, never stored
}

// LoginResult contains the output of a successful login.
type LoginResult struct {
	UserID string
	DEK    []byte // unwrapped DEK, for per-user blob encryption
}

// Authenticator defines the interface for user authentication methods.
// Each method (password, OAuth, LDAP, etc.) implements this interface.
type Authenticator interface {
	// SignUp creates a new user account. Returns the user ID and a one-time
	// recovery key.
	SignUp(username string, password []byte) (*SignUpResult, error)

	// Login authenticates a user and returns a session-ready user ID and the
	// unwrapped DEK for per-user blob encryption.
	Login(username string, password []byte) (*LoginResult, error)
}

// PasswordAuthenticator implements password-based authentication using
// Argon2id for key derivation (TECHNICAL-DECISIONS §1.11).
type PasswordAuthenticator struct {
	store UserStore
}

// NewPasswordAuthenticator returns a PasswordAuthenticator backed by the
// given user store.
func NewPasswordAuthenticator(store UserStore) *PasswordAuthenticator {
	return &PasswordAuthenticator{store: store}
}

// SignUp creates a new user with the given username and password.
// It generates a salt, derives the password-KEK using Argon2id, generates
// a random DEK and recovery key, wraps the DEK, and stores everything.
func (a *PasswordAuthenticator) SignUp(username string, password []byte) (*SignUpResult, error) {
	if username == "" {
		return nil, fmt.Errorf("auth: username is required")
	}
	if len(password) == 0 {
		return nil, fmt.Errorf("auth: password is required")
	}

	// Check if username already exists.
	if _, err := a.store.GetUser(username); err == nil {
		return nil, fmt.Errorf("auth: username already exists")
	}

	// Generate random salt.
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("auth: generate salt: %w", err)
	}

	// Derive password-KEK using Argon2id.
	passwordKEK := argon2.IDKey(password, salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Generate random DEK.
	dek := make([]byte, dekLen)
	if _, err := rand.Read(dek); err != nil {
		return nil, fmt.Errorf("auth: generate DEK: %w", err)
	}

	// Wrap DEK with password-KEK.
	wrappedDEK, err := wrapKey(dek, passwordKEK)
	if err != nil {
		return nil, fmt.Errorf("auth: wrap DEK with password-KEK: %w", err)
	}

	// Generate recovery key (128-bit random, base32 encoded).
	recoveryKey := generateRecoveryKey()

	// Derive recovery-KEK from recovery key.
	recoveryKEK := deriveRecoveryKEK(recoveryKey)

	// Wrap DEK with recovery-KEK.
	wrappedRecovery, err := wrapKey(dek, recoveryKEK)
	if err != nil {
		return nil, fmt.Errorf("auth: wrap DEK with recovery-KEK: %w", err)
	}

	// Store user data.
	userData := &UserData{
		Salt:            salt,
		PasswordHash:    passwordKEK,
		WrappedDEK:      wrappedDEK,
		WrappedRecovery: wrappedRecovery,
	}
	if err := a.store.PutUser(username, userData); err != nil {
		return nil, fmt.Errorf("auth: store user: %w", err)
	}

	return &SignUpResult{
		UserID:      "user:" + username,
		RecoveryKey: recoveryKey,
	}, nil
}

// Login authenticates a user with the given username and password.
// It re-derives the password-KEK, verifies it against the stored hash,
// and returns wrapped DEK material.
func (a *PasswordAuthenticator) Login(username string, password []byte) (*LoginResult, error) {
	if username == "" {
		return nil, fmt.Errorf("auth: username is required")
	}
	if len(password) == 0 {
		return nil, fmt.Errorf("auth: password is required")
	}

	// Look up user.
	userData, err := a.store.GetUser(username)
	if err != nil {
		return nil, fmt.Errorf("auth: invalid credentials")
	}

	// Re-derive password-KEK using stored salt.
	passwordKEK := argon2.IDKey(password, userData.Salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLen)

	// Verify password hash.
	if !bytesEqual(passwordKEK, userData.PasswordHash) {
		return nil, fmt.Errorf("auth: invalid credentials")
	}

	// Unwrap the DEK using the password-KEK.
	dek, err := unwrapKey(userData.WrappedDEK, passwordKEK)
	if err != nil {
		return nil, fmt.Errorf("auth: unwrap DEK: %w", err)
	}

	return &LoginResult{
		UserID: "user:" + username,
		DEK:    dek,
	}, nil
}

// wrapKey encrypts a key using AES-GCM with the given wrapping key.
func wrapKey(key, wrappingKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(wrappingKey)
	if err != nil {
		return nil, fmt.Errorf("wrap: aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("wrap: gcm: %w", err)
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("wrap: generate nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, key, nil), nil
}

// unwrapKey decrypts a wrapped key using AES-GCM with the given unwrapping key.
func unwrapKey(wrapped, unwrappingKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(unwrappingKey)
	if err != nil {
		return nil, fmt.Errorf("unwrap: aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("unwrap: gcm: %w", err)
	}
	nonceSize := aead.NonceSize()
	if len(wrapped) < nonceSize {
		return nil, fmt.Errorf("unwrap: ciphertext too short")
	}
	nonce, ciphertext := wrapped[:nonceSize], wrapped[nonceSize:]
	return aead.Open(nil, nonce, ciphertext, nil)
}

// bytesEqual returns true if two byte slices are equal.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// generateRecoveryKey generates a 128-bit random recovery key encoded as
// base32 (26 characters, TECHNICAL-DECISIONS §1.11).
func generateRecoveryKey() string {
	b := make([]byte, 16) // 128 bits
	if _, err := rand.Read(b); err != nil {
		panic("auth: failed to generate recovery key: " + err.Error())
	}
	return base32Encode(b)
}

// base32Encode encodes bytes to base32 (RFC 4648, no padding).
func base32Encode(data []byte) string {
	var buf strings.Builder
	for i := 0; i < len(data); i += 5 {
		// Pad to 5 bytes.
		var chunk [5]byte
		n := copy(chunk[:], data[i:])
		_ = n

		// Encode 5 bytes to 8 base32 characters.
		buf.WriteByte(base32Alphabet[(chunk[0]>>3)&0x1F])
		buf.WriteByte(base32Alphabet[((chunk[0]<<2)|(chunk[1]>>6))&0x1F])
		buf.WriteByte(base32Alphabet[(chunk[1]>>1)&0x1F])
		buf.WriteByte(base32Alphabet[((chunk[1]<<4)|(chunk[2]>>4))&0x1F])
		buf.WriteByte(base32Alphabet[((chunk[2]<<1)|(chunk[3]>>7))&0x1F])
		buf.WriteByte(base32Alphabet[(chunk[3]>>2)&0x1F])
		buf.WriteByte(base32Alphabet[((chunk[3]<<3)|(chunk[4]>>5))&0x1F])
		buf.WriteByte(base32Alphabet[chunk[4]&0x1F])
	}
	return buf.String()
}

// deriveRecoveryKEK derives a key-encryption key from the recovery key
// using Argon2id (TECHNICAL-DECISIONS §1.11).
func deriveRecoveryKEK(recoveryKey string) []byte {
	// Use a fixed salt derived from the recovery key itself. This is fine
	// because the recovery key is already high-entropy (128-bit random).
	salt := argon2.IDKey([]byte(recoveryKey), nil, 1, 64*1024, 1, 32)
	return salt
}

// UserStore is the persistence interface for user authentication data.
type UserStore interface {
	GetUser(username string) (*UserData, error)
	PutUser(username string, data *UserData) error
}

// UserData holds the authentication material for a user.
type UserData struct {
	Salt            []byte // random salt for Argon2id
	PasswordHash    []byte // Argon2id(password, salt) — the password-KEK
	WrappedDEK      []byte // DEK wrapped by password-KEK
	WrappedRecovery []byte // DEK wrapped by recovery-KEK
}

// MemoryUserStore is an in-memory implementation of UserStore.
type MemoryUserStore struct {
	users map[string]*UserData
}

// NewMemoryUserStore returns a new in-memory user store.
func NewMemoryUserStore() *MemoryUserStore {
	return &MemoryUserStore{users: make(map[string]*UserData)}
}

func (s *MemoryUserStore) GetUser(username string) (*UserData, error) {
	data, ok := s.users[username]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}
	return data, nil
}

func (s *MemoryUserStore) PutUser(username string, data *UserData) error {
	s.users[username] = data
	return nil
}

// CompositeAuthenticator routes auth requests to method-specific
// authenticators via a map-based registry. No ordering or fallback.
type CompositeAuthenticator struct {
	methods map[string]Authenticator
}

// Get returns the Authenticator for the given method, or nil if not found.
func (c *CompositeAuthenticator) Get(method string) (Authenticator, bool) {
	a, ok := c.methods[method]
	return a, ok
}

// NewAuthenticators creates a CompositeAuthenticator from the configured
// method names. Currently only "password" is supported.
func NewAuthenticators(methods []string, userStore UserStore) (*CompositeAuthenticator, error) {
	m := make(map[string]Authenticator, len(methods))
	for _, name := range methods {
		switch name {
		case "password":
			m[name] = NewPasswordAuthenticator(userStore)
		default:
			return nil, fmt.Errorf("auth: unknown method %q", name)
		}
	}
	return &CompositeAuthenticator{methods: m}, nil
}
