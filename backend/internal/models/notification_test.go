package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNotification_BeforeCreate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	assert.NoError(t, err)
	db.AutoMigrate(&Notification{})

	// Case 1: ID is empty, should be generated
	n1 := &Notification{Title: "Test", Message: "Test Message"}
	err = db.Create(n1).Error
	assert.NoError(t, err)
	assert.NotEmpty(t, n1.ID)

	// Case 2: ID is provided, should be kept
	id := "123e4567-e89b-12d3-a456-426614174000"
	n2 := &Notification{ID: id, Title: "Test 2", Message: "Test Message 2"}
	err = db.Create(n2).Error
	assert.NoError(t, err)
	assert.Equal(t, id, n2.ID)
}
