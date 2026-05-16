package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProxyGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyGroup{}, &models.ProxyHost{}))
	return db
}

func TestProxyGroupService_Create(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)

	group := &models.ProxyGroup{Name: "Production", Color: "#ff0000"}
	err := svc.Create(group)
	require.NoError(t, err)
	assert.NotEmpty(t, group.UUID)
	assert.NotZero(t, group.ID)
}

func TestProxyGroupService_Create_EmptyName(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)

	err := svc.Create(&models.ProxyGroup{Name: ""})
	assert.ErrorIs(t, err, ErrProxyGroupNameEmpty)
}

func TestProxyGroupService_List(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)

	require.NoError(t, svc.Create(&models.ProxyGroup{Name: "Zebra"}))
	require.NoError(t, svc.Create(&models.ProxyGroup{Name: "Alpha"}))

	groups, err := svc.List()
	require.NoError(t, err)
	require.Len(t, groups, 2)
	assert.Equal(t, "Alpha", groups[0].Name)
	assert.Equal(t, "Zebra", groups[1].Name)
}

func TestProxyGroupService_GetByUUID(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)

	created := &models.ProxyGroup{Name: "Test Group"}
	require.NoError(t, svc.Create(created))

	t.Run("found", func(t *testing.T) {
		got, err := svc.GetByUUID(created.UUID)
		require.NoError(t, err)
		assert.Equal(t, created.Name, got.Name)
	})

	t.Run("not found", func(t *testing.T) {
		_, err := svc.GetByUUID("non-existent-uuid")
		assert.ErrorIs(t, err, ErrProxyGroupNotFound)
	})
}

func TestProxyGroupService_Update(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)

	group := &models.ProxyGroup{Name: "Original", Color: "#111111"}
	require.NoError(t, svc.Create(group))

	group.Name = "Updated"
	group.Description = "New description"
	group.Color = "#222222"
	require.NoError(t, svc.Update(group))

	got, err := svc.GetByUUID(group.UUID)
	require.NoError(t, err)
	assert.Equal(t, "Updated", got.Name)
	assert.Equal(t, "New description", got.Description)
	assert.Equal(t, "#222222", got.Color)
}

func TestProxyGroupService_Delete_ClearsHostAssignments(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)

	group := &models.ProxyGroup{Name: "ToDelete"}
	require.NoError(t, svc.Create(group))

	host := &models.ProxyHost{
		UUID:          "test-host-uuid",
		DomainNames:   "example.com",
		ForwardScheme: "http",
		ForwardHost:   "localhost",
		ForwardPort:   80,
		ProxyGroupID:  &group.ID,
	}
	require.NoError(t, db.Create(host).Error)

	require.NoError(t, svc.Delete(group.ID))

	var updated models.ProxyHost
	require.NoError(t, db.First(&updated, host.ID).Error)
	assert.Nil(t, updated.ProxyGroupID)

	var count int64
	db.Model(&models.ProxyGroup{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestProxyGroupService_GetHostCount(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)

	group := &models.ProxyGroup{Name: "Counted"}
	require.NoError(t, svc.Create(group))

	for i := range 3 {
		h := &models.ProxyHost{
			UUID:          "host-uuid-" + string(rune('a'+i)),
			DomainNames:   "test.com",
			ForwardScheme: "http",
			ForwardHost:   "localhost",
			ForwardPort:   80 + i,
			ProxyGroupID:  &group.ID,
		}
		require.NoError(t, db.Create(h).Error)
	}

	count, err := svc.GetHostCount(group.ID)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func TestProxyGroupService_List_DBError(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)
	require.NoError(t, db.Migrator().DropTable(&models.ProxyGroup{}))
	_, err := svc.List()
	assert.Error(t, err)
}

func TestProxyGroupService_GetByUUID_DBError(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)
	require.NoError(t, db.Migrator().DropTable(&models.ProxyGroup{}))
	_, err := svc.GetByUUID("some-uuid")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, ErrProxyGroupNotFound)
}

func TestProxyGroupService_Delete_TransactionError(t *testing.T) {
	db := setupProxyGroupTestDB(t)
	svc := NewProxyGroupService(db)
	group := &models.ProxyGroup{Name: "FailDelete"}
	require.NoError(t, svc.Create(group))
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))
	err := svc.Delete(group.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unassign proxy hosts")
}
