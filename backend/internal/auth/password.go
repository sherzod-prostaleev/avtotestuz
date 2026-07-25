package auth

import (
	"errors"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

const (
	minPasswordLen = 8
	bcryptCost     = 12
)

var (
	ErrWeakPassword   = errors.New("weak password")
	ErrInvalidCreds   = errors.New("invalid phone or password")
	ErrPhoneTaken     = errors.New("phone already registered")
	ErrPasswordNotSet = errors.New("password not set")
	ErrPasswordSet    = errors.New("password already set")
)

// HashPassword returns a bcrypt hash of password (cost 12). Never log the input.
func HashPassword(password string) (string, error) {
	if utf8.RuneCountInString(password) < minPasswordLen {
		return "", ErrWeakPassword
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password with a stored bcrypt hash.
func CheckPassword(hash, password string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
