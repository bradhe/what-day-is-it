package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanPhoneNumberRemovesSpecialCharacters(t *testing.T) {
	assert.Equal(t, "+15554443333", CleanPhoneNumber("+1 555 444 3333"))
	assert.Equal(t, "+15554443333", CleanPhoneNumber("+1 555 hello world 444 3333"))
}

func TestCleanPhoneNumberAddsDefaultCountryCodeIfMissingAndValid(t *testing.T) {
	assert.Equal(t, "+15554443333", CleanPhoneNumber("555 444 3333"))
	assert.Equal(t, "+15554443333", CleanPhoneNumber("1 555 444 3333"))
	assert.Equal(t, "+15554443333", CleanPhoneNumber("+1 555 444 3333"))
	assert.Equal(t, "+445554443333", CleanPhoneNumber("44 555 444 33 33"))

	// If the phone number is invalid we just clean it up.
	assert.Equal(t, "555 444 333", CleanPhoneNumber("555 444 333"))
}
func TestCleanPhoneNumberLeavesTotallyInvalidInputAlone(t *testing.T) {
	assert.Equal(t, "", CleanPhoneNumber(""))
	assert.Equal(t, "Hello, World!", CleanPhoneNumber("Hello, World!"))
}

func TestIsCleanPhoneNumber(t *testing.T) {
	assert.True(t, IsCleanPhoneNumber("+445554443333"))
	assert.True(t, IsCleanPhoneNumber("+15554443333"))
	assert.True(t, IsCleanPhoneNumber("+3565554443333"))

	// missing country code
	assert.False(t, IsCleanPhoneNumber("+5554443333"))
	assert.False(t, IsCleanPhoneNumber("Hello, World!"))
}
