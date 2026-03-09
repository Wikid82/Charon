package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNotificationService_TemplateCRUD(t *testing.T) {
	t.Parallel()

	db, err := gorm.Open(sqlite.Open("file:"+uuid.NewString()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationTemplate{}))

	svc := NewNotificationService(db, nil)

	tmpl := &models.NotificationTemplate{
		Name:        "Custom",
		Description: "initial description",
		Config:      `{"message":"hello"}`,
		Template:    "custom",
	}

	require.NoError(t, svc.CreateTemplate(tmpl))
	require.NotEmpty(t, tmpl.ID)

	fetched, err := svc.GetTemplate(tmpl.ID)
	require.NoError(t, err)
	assert.Equal(t, tmpl.Name, fetched.Name)
	assert.Equal(t, tmpl.Description, fetched.Description)

	tmpl.Description = "updated description"
	require.NoError(t, svc.UpdateTemplate(tmpl))

	list, err := svc.ListTemplates()
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "updated description", list[0].Description)

	require.NoError(t, svc.DeleteTemplate(tmpl.ID))
	list, err = svc.ListTemplates()
	require.NoError(t, err)
	assert.Empty(t, list)
}
