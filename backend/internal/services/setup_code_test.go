package services

import (
	"encoding/base32"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newSetupCodeTestDB(t *testing.T, migrate bool) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "setup-code.db") + "?_pragma=busy_timeout(10000)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	if migrate {
		require.NoError(t, db.AutoMigrate(&models.User{}))
	}
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestResolveSetupCode_GeneratesBase32CodeOfAtLeastMinLength(t *testing.T) {
	first, err := ResolveSetupCode("")
	require.NoError(t, err)
	second, err := ResolveSetupCode("")
	require.NoError(t, err)

	assert.GreaterOrEqual(t, len(first), SetupCodeMinLength)
	assert.NotEqual(t, first, second, "generated codes must not repeat")
	_, decodeErr := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(first)
	assert.NoError(t, decodeErr)
}

func TestResolveSetupCode_PresetAtMinLengthIsUsed(t *testing.T) {
	preset := strings.Repeat("a", SetupCodeMinLength)
	code, err := ResolveSetupCode(preset)
	require.NoError(t, err)
	assert.Equal(t, preset, code)
}

func TestResolveSetupCode_PresetBelowMinLengthIsRejected(t *testing.T) {
	_, err := ResolveSetupCode(strings.Repeat("a", SetupCodeMinLength-1))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "aaaa", "the error must not echo the preset")
}

func TestSetupCodeGuard_AcceptsOnlyTheIssuedCode(t *testing.T) {
	code := strings.Repeat("c", SetupCodeMinLength)
	guard := NewSetupCodeGuard(code)

	assert.True(t, guard.Active())
	assert.True(t, guard.Verify(code))
	assert.False(t, guard.Verify(""))
	assert.False(t, guard.Verify(code+"x"))
	assert.False(t, guard.Verify(strings.ToUpper(code)))
}

func TestSetupCodeGuard_RetainsOnlyADigest(t *testing.T) {
	code := strings.Repeat("d", SetupCodeMinLength)
	guard := NewSetupCodeGuard(code)

	assert.Len(t, guard.digest, 32)
	assert.NotEqual(t, []byte(code), guard.digest)
}

func TestSetupCodeGuard_EmptyOrNilAcceptsNothing(t *testing.T) {
	var nilGuard *SetupCodeGuard
	empty := NewSetupCodeGuard("")

	for _, g := range []*SetupCodeGuard{nilGuard, empty} {
		assert.False(t, g.Active())
		assert.False(t, g.Verify(""))
		assert.False(t, g.Verify("anything"))
		g.Discard() // must not panic
	}
}

func TestSetupCodeGuard_DiscardEndsAcceptance(t *testing.T) {
	code := strings.Repeat("e", SetupCodeMinLength)
	guard := NewSetupCodeGuard(code)
	require.True(t, guard.Verify(code))

	guard.Discard()

	assert.False(t, guard.Active())
	assert.False(t, guard.Verify(code))
}

func TestSetupCodeGuard_ConcurrentVerifyAndDiscardIsSafe(t *testing.T) {
	code := strings.Repeat("f", SetupCodeMinLength)
	guard := NewSetupCodeGuard(code)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = guard.Verify(code)
			_ = guard.Active()
		}()
		go func() {
			defer wg.Done()
			guard.Discard()
		}()
	}
	wg.Wait()

	assert.False(t, guard.Active())
}

func TestIssueStartupSetupCode_EmptyTableGeneratesCode(t *testing.T) {
	db := newSetupCodeTestDB(t, true)

	guard, issue, err := IssueStartupSetupCode(db, "")
	require.NoError(t, err)
	assert.True(t, issue.Pending)
	assert.True(t, issue.Generated)
	require.NotEmpty(t, issue.Code, "a generated code is returned so the caller can show it once")
	assert.True(t, guard.Verify(issue.Code))
}

func TestIssueStartupSetupCode_EmptyTableUsesPresetWithoutReturningIt(t *testing.T) {
	db := newSetupCodeTestDB(t, true)
	preset := strings.Repeat("p", SetupCodeMinLength)

	guard, issue, err := IssueStartupSetupCode(db, preset)
	require.NoError(t, err)
	assert.True(t, issue.Pending)
	assert.False(t, issue.Generated, "a preset is reported as operator-supplied, not generated")
	assert.Empty(t, issue.Code, "the preset must not be echoed back to the caller")
	assert.True(t, guard.Verify(preset))
}

func TestIssueStartupSetupCode_PopulatedTableIssuesNothing(t *testing.T) {
	db := newSetupCodeTestDB(t, true)
	require.NoError(t, db.Create(&models.User{Email: "admin@example.com", Name: "Admin", Enabled: true}).Error)

	guard, issue, err := IssueStartupSetupCode(db, "")
	require.NoError(t, err)
	assert.Equal(t, SetupCodeIssue{}, issue)
	assert.False(t, guard.Active())
}

func TestIssueStartupSetupCode_InvalidPresetRejectedEvenWhenPopulated(t *testing.T) {
	db := newSetupCodeTestDB(t, true)
	require.NoError(t, db.Create(&models.User{Email: "admin@example.com", Name: "Admin", Enabled: true}).Error)

	_, _, err := IssueStartupSetupCode(db, "short")
	require.Error(t, err)
}

func TestIssueStartupSetupCode_CountFailureIsReturned(t *testing.T) {
	db := newSetupCodeTestDB(t, false) // no users table

	_, _, err := IssueStartupSetupCode(db, "")
	require.Error(t, err)
}
