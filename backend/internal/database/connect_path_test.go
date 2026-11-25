package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConnect_RejectsPathWithQueryMarker(t *testing.T) {
	db, err := Connect("data?unsafe.db")

	assert.Nil(t, db)
	assert.ErrorContains(t, err, "build database connection string")
}
