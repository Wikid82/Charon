package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestDomain_BeforeCreate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)
	db.AutoMigrate(&Domain{})

	// Case 1: UUID is empty, should be generated
	d1 := &Domain{Name: "example.com"}
	err = db.Create(d1).Error
	assert.NoError(t, err)
	assert.NotEmpty(t, d1.UUID)

	// Case 2: UUID is provided, should be kept
	uuid := "123e4567-e89b-12d3-a456-426614174000"
	d2 := &Domain{Name: "test.com", UUID: uuid}
	err = db.Create(d2).Error
	assert.NoError(t, err)
	assert.Equal(t, uuid, d2.UUID)
}
