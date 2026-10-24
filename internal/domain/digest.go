package domain

import (
	"encoding/hex"
	"errors"
	"strings"
)

// ValidateSHA256 validates a hexadecimal SHA-256 digest.
func ValidateSHA256(value string) error {
	if len(value) != 64 {
		return errors.New("SHA-256 digest must contain 64 hexadecimal characters")
	}
	if _, err := hex.DecodeString(value); err != nil {
		return errors.New("SHA-256 digest must be hexadecimal")
	}
	return nil
}

// ValidateLowercaseSHA256 validates a canonical lowercase hexadecimal SHA-256 digest.
func ValidateLowercaseSHA256(value string) error {
	if err := ValidateSHA256(value); err != nil {
		return err
	}
	if value != strings.ToLower(value) {
		return errors.New("SHA-256 digest must use lowercase hexadecimal")
	}
	return nil
}
