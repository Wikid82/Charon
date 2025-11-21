package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUser_SetPassword(t *testing.T) {
	u := &User{}
	err := u.SetPassword("password123")
	assert.NoError(t, err)
	assert.NotEmpty(t, u.PasswordHash)
	assert.NotEqual(t, "password123", u.PasswordHash)
}

func TestUser_CheckPassword(t *testing.T) {
	u := &User{}
	_ = u.SetPassword("password123")

	assert.True(t, u.CheckPassword("password123"))
	assert.False(t, u.CheckPassword("wrongpassword"))
}
