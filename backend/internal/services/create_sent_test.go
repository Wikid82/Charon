package services

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// sentProbe has one defaulted field of each kind the create helper handles, one field without a default and one field
// that is not serialised.
type sentProbe struct {
	ID      uint   `gorm:"primaryKey"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled" gorm:"default:true"`
	Mode    string `json:"mode" gorm:"default:block"`
	Count   int    `json:"count" gorm:"default:5"`
	Plain   bool   `json:"plain"`
	Hidden  string `json:"-" gorm:"default:hidden"`
}

// newSentProbeDB opens an in-memory database with the probe table.
func newSentProbeDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:create_sent_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&sentProbe{}))
	return db
}

// TestSentDefaultColumns_ListsSentDefaultedFieldsOnly describes the column list: only defaulted fields the client sent
// are listed, a kind filter narrows it, and a field that is not serialised is never listed.
func TestSentDefaultColumns_ListsSentDefaultedFieldsOnly(t *testing.T) {
	db := newSentProbeDB(t)

	all, err := SentDefaultColumns(db, &sentProbe{}, []string{"enabled", "mode", "count", "plain", "name", "-", "missing"})
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"enabled", "mode", "count"}, all)

	bools, err := SentDefaultColumns(db, &sentProbe{}, []string{"enabled", "mode", "count"}, reflect.Bool)
	require.NoError(t, err)
	assert.Equal(t, []string{"enabled"}, bools)

	none, err := SentDefaultColumns(db, &sentProbe{}, nil)
	require.NoError(t, err)
	assert.Empty(t, none)
}

// TestSentDefaultColumns_SchemaErrorIsReturned describes a model that cannot be parsed as a schema.
func TestSentDefaultColumns_SchemaErrorIsReturned(t *testing.T) {
	db := newSentProbeDB(t)

	_, err := SentDefaultColumns(db, 42, []string{"enabled"})

	require.Error(t, err)
}

// TestJSONFieldName_SkipsUnserialisedFields describes the JSON name lookup used by the column list.
func TestJSONFieldName_SkipsUnserialisedFields(t *testing.T) {
	db := newSentProbeDB(t)
	stmt := &gorm.Statement{DB: db}
	require.NoError(t, stmt.Parse(&sentProbe{}))

	assert.Equal(t, "enabled", jsonFieldName(stmt.Schema.LookUpField("enabled")))
	assert.Equal(t, "", jsonFieldName(stmt.Schema.LookUpField("hidden")))
}

// TestCreateKeepingSent_StoresSentValuesInRowAndStruct describes the stored-as-sent rule: false, "" and 0 that the
// client sent replace the defaults in the row and in the struct; a column that was not sent keeps its default.
func TestCreateKeepingSent_StoresSentValuesInRowAndStruct(t *testing.T) {
	db := newSentProbeDB(t)
	probe := sentProbe{Name: "sent", Enabled: false, Mode: "", Count: 0}

	require.NoError(t, CreateKeepingSent(db, &probe, []string{"enabled", "mode", "count"}))

	assert.False(t, probe.Enabled)
	assert.Equal(t, "", probe.Mode)
	assert.Equal(t, 0, probe.Count)
	var stored sentProbe
	require.NoError(t, db.First(&stored, probe.ID).Error)
	assert.False(t, stored.Enabled)
	assert.Equal(t, "", stored.Mode)
	assert.Equal(t, 0, stored.Count)
}

// TestCreateKeepingSent_UnsentColumnsKeepTheirDefault describes a column the client did not send: GORM's default applies
// to it, as it does without the helper.
func TestCreateKeepingSent_UnsentColumnsKeepTheirDefault(t *testing.T) {
	db := newSentProbeDB(t)
	probe := sentProbe{Name: "unsent", Enabled: false}

	require.NoError(t, CreateKeepingSent(db, &probe, nil))

	var stored sentProbe
	require.NoError(t, db.First(&stored, probe.ID).Error)
	assert.True(t, stored.Enabled, "without a sent column the default still applies")
}

// TestCreateKeepingSent_IgnoresColumnsOutsideTheSchema describes a sent column the model does not have: it is skipped and
// the row is created.
func TestCreateKeepingSent_IgnoresColumnsOutsideTheSchema(t *testing.T) {
	db := newSentProbeDB(t)
	probe := sentProbe{Name: "unknown-column"}

	require.NoError(t, CreateKeepingSent(db, &probe, []string{"no_such_column"}))

	assert.NotZero(t, probe.ID)
}

// TestCreateKeepingSent_ErrorsAreReturned describes the failure paths: a schema that cannot be parsed, an insert that
// fails, and a restore write that fails.
func TestCreateKeepingSent_ErrorsAreReturned(t *testing.T) {
	t.Run("schema", func(t *testing.T) {
		db := newSentProbeDB(t)
		require.Error(t, CreateKeepingSent(db, 42, []string{"enabled"}))
	})
	t.Run("insert", func(t *testing.T) {
		db, err := gorm.Open(sqlite.Open("file:create_sent_no_table?mode=memory&cache=shared"), &gorm.Config{})
		require.NoError(t, err)
		probe := sentProbe{Name: "no-table"}
		require.Error(t, CreateKeepingSent(db, &probe, []string{"enabled"}))
	})
	t.Run("restore", func(t *testing.T) {
		db := newSentProbeDB(t)
		gormfault.InjectNth(t, db, gormfault.Update, "sent_probes", 1, nil)
		probe := sentProbe{Name: "restore-fails", Enabled: false}
		require.Error(t, CreateKeepingSent(db, &probe, []string{"enabled"}))
	})
}
