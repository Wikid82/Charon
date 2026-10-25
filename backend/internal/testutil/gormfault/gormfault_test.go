package gormfault_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

type item struct {
	ID    uint `gorm:"primaryKey"`
	Name  string
	Value string
}

type other struct {
	ID   uint `gorm:"primaryKey"`
	Name string
}

func newFaultDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&item{}, &other{}))
	require.NoError(t, db.Create(&item{Name: "a", Value: "1"}).Error)
	require.NoError(t, db.Create(&item{Name: "b", Value: "2"}).Error)
	require.NoError(t, db.Create(&other{Name: "a"}).Error)
	return db
}

func TestOpString(t *testing.T) {
	assert.Equal(t, "query", gormfault.Query.String())
	assert.Equal(t, "update", gormfault.Update.String())
	assert.Equal(t, "delete", gormfault.Delete.String())
}

func TestInject_QueryFailsEveryMatchingStatementOnly(t *testing.T) {
	db := newFaultDB(t)
	gormfault.Inject(t, db, gormfault.Query, "items", nil)

	var items []item
	err := db.Find(&items).Error
	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err))

	var others []other
	require.NoError(t, db.Find(&others).Error, "statements on other tables still run")
	assert.Len(t, others, 1)
}

func TestInjectNth_FailsOnlyTheNthMatch(t *testing.T) {
	db := newFaultDB(t)
	gormfault.InjectNth(t, db, gormfault.Query, "items", 2, gormfault.VarIs("a"))

	var first []item
	require.NoError(t, db.Where("name = ?", "a").Find(&first).Error, "match 1 runs")

	var second []item
	err := db.Where("name = ?", "a").Find(&second).Error
	require.Error(t, err, "match 2 fails")
	assert.True(t, gormfault.IsInjected(err))

	var third []item
	require.NoError(t, db.Where("name = ?", "a").Find(&third).Error, "match 3 runs again")
}

func TestInject_VarIsMatchesOnlyTheBoundValue(t *testing.T) {
	db := newFaultDB(t)
	gormfault.Inject(t, db, gormfault.Query, "items", gormfault.VarIs("b"))

	var a []item
	require.NoError(t, db.Where("name = ?", "a").Find(&a).Error, "a statement without the value runs")

	var b []item
	require.Error(t, db.Where("name = ?", "b").Find(&b).Error)
}

func TestInject_VarIsDoesNotMatchStatementsWithoutWhere(t *testing.T) {
	db := newFaultDB(t)
	gormfault.Inject(t, db, gormfault.Query, "items", gormfault.VarIs("a"))

	var all []item
	require.NoError(t, db.Find(&all).Error, "a statement without a WHERE clause is not matched")
	assert.Len(t, all, 2)
}

func TestInject_UpdateSetsMatchesColumnMapOnly(t *testing.T) {
	db := newFaultDB(t)
	gormfault.Inject(t, db, gormfault.Update, "items", gormfault.UpdateSets("value"))

	err := db.Model(&item{}).Where("name = ?", "a").Update("value", "9").Error
	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err))

	require.NoError(t, db.Model(&item{}).Where("name = ?", "a").Update("name", "renamed").Error,
		"a column map without the key runs")

	var row item
	require.NoError(t, db.Where("name = ?", "renamed").First(&row).Error)
	assert.Equal(t, "1", row.Value)
}

func TestInject_UpdateSetsIgnoresNonMapUpdates(t *testing.T) {
	db := newFaultDB(t)
	gormfault.Inject(t, db, gormfault.Update, "items", gormfault.UpdateSets("value"))

	var row item
	require.NoError(t, db.Where("name = ?", "a").First(&row).Error)
	row.Name = "saved"
	require.NoError(t, db.Save(&row).Error, "a struct save is not a column map and runs")
}

func TestInject_DeleteFailsMatchingDelete(t *testing.T) {
	db := newFaultDB(t)
	gormfault.Inject(t, db, gormfault.Delete, "items", gormfault.VarIs("b"))

	err := db.Where("name = ?", "b").Delete(&item{}).Error
	require.Error(t, err)
	assert.True(t, gormfault.IsInjected(err))

	var remaining []item
	require.NoError(t, db.Where("name = ?", "b").Find(&remaining).Error)
	assert.Len(t, remaining, 1, "the failed delete removed nothing")
}

func TestIsInjected_FalseForOrdinaryErrors(t *testing.T) {
	assert.False(t, gormfault.IsInjected(nil))
	assert.False(t, gormfault.IsInjected(gorm.ErrRecordNotFound))
}
