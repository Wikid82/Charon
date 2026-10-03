package services

import (
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const loginTestPassword = "password123"

// setupLoginTestDB opens a file-backed database with the same SQLite driver the
// application uses.
func setupLoginTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "login.db") + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	t.Cleanup(func() {
		if sqlDB, e := db.DB(); e == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func newLoginTestService(t *testing.T) (*AuthService, *gorm.DB, *models.User) {
	t.Helper()
	db := setupLoginTestDB(t)
	svc := NewAuthService(db, config.Config{JWTSecret: "test-secret"})
	user, err := svc.Register("user@example.com", loginTestPassword, "User")
	require.NoError(t, err)
	return svc, db, user
}

func reloadUser(t *testing.T, db *gorm.DB, id uint) models.User {
	t.Helper()
	var u models.User
	require.NoError(t, db.First(&u, id).Error)
	return u
}

func wrongLogins(t *testing.T, svc *AuthService, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, err := svc.Login("user@example.com", "wrong-password")
		require.ErrorIs(t, err, ErrInvalidLogin)
	}
}

func TestLogin_UniformFailureResponse(t *testing.T) {
	svc, db, user := newLoginTestService(t)

	disabled, err := svc.Register("disabled@example.com", loginTestPassword, "Disabled")
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", disabled.ID).Update("enabled", false).Error)

	locked, err := svc.Register("locked@example.com", loginTestPassword, "Locked")
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", locked.ID).
		Update("locked_until", time.Now().UTC().Add(time.Hour)).Error)

	cases := map[string]struct{ email, password string }{
		"unknown account":   {"nobody@example.com", loginTestPassword},
		"wrong password":    {user.Email, "wrong-password"},
		"disabled account":  {"disabled@example.com", loginTestPassword},
		"locked correct pw": {"locked@example.com", loginTestPassword},
		"locked wrong pw":   {"locked@example.com", "wrong-password"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			token, err := svc.Login(tc.email, tc.password)
			assert.Empty(t, token)
			require.ErrorIs(t, err, ErrInvalidLogin)
			assert.Equal(t, "invalid credentials", err.Error())
		})
	}
}

func TestLogin_UnknownAccountRunsPasswordCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("duration comparison skipped in short mode")
	}
	svc, _, _ := newLoginTestService(t)
	placeholderHash() // generate outside the measured section

	median := func(email string) time.Duration {
		const n = 5
		d := make([]time.Duration, 0, n)
		for i := 0; i < n; i++ {
			start := time.Now()
			_, _ = svc.Login(email, "wrong-password")
			d = append(d, time.Since(start))
		}
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		return d[n/2]
	}
	// Use a separate account for the wrong-password path so the first account's
	// lock state does not influence the comparison.
	_, err := svc.Register("second@example.com", loginTestPassword, "Second")
	require.NoError(t, err)

	known := median("second@example.com")
	unknown := median("nobody@example.com")
	// The unknown path skips one small database write; it must still be dominated
	// by a bcrypt comparison.
	assert.GreaterOrEqual(t, float64(unknown), 0.5*float64(known),
		"unknown=%v known=%v", unknown, known)
}

func TestPlaceholderHash_MatchesStoredHashCost(t *testing.T) {
	var u models.User
	require.NoError(t, u.SetPassword("anything"))
	stored, err := bcrypt.Cost([]byte(u.PasswordHash))
	require.NoError(t, err)
	dummy, err := bcrypt.Cost(placeholderHash())
	require.NoError(t, err)
	assert.Equal(t, stored, dummy)
}

func TestLogin_LockAfterMaxAttempts(t *testing.T) {
	svc, db, user := newLoginTestService(t)

	wrongLogins(t, svc, MaxFailedLoginAttempts-1)
	u := reloadUser(t, db, user.ID)
	assert.Equal(t, MaxFailedLoginAttempts-1, u.FailedLoginAttempts)
	assert.Nil(t, u.LockedUntil)

	wrongLogins(t, svc, 1)
	u = reloadUser(t, db, user.ID)
	assert.Equal(t, MaxFailedLoginAttempts, u.FailedLoginAttempts)
	require.NotNil(t, u.LockedUntil)
	assert.WithinDuration(t, time.Now().Add(LockDuration), *u.LockedUntil, 10*time.Second)

	// Correct password during the lock still fails and changes nothing.
	_, err := svc.Login(user.Email, loginTestPassword)
	require.ErrorIs(t, err, ErrInvalidLogin)
	after := reloadUser(t, db, user.ID)
	assert.Equal(t, u.FailedLoginAttempts, after.FailedLoginAttempts)
	assert.True(t, u.LockedUntil.Equal(*after.LockedUntil))
}

func TestLogin_AttemptsWhileLockedDoNotCountOrExtend(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	wrongLogins(t, svc, MaxFailedLoginAttempts)
	locked := reloadUser(t, db, user.ID)
	require.NotNil(t, locked.LockedUntil)

	wrongLogins(t, svc, 3)
	after := reloadUser(t, db, user.ID)
	assert.Equal(t, locked.FailedLoginAttempts, after.FailedLoginAttempts)
	assert.True(t, locked.LockedUntil.Equal(*after.LockedUntil), "lock must not be extended")
}

func TestLogin_CounterRestartsAfterLockExpiry(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	wrongLogins(t, svc, MaxFailedLoginAttempts)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).
		Update("locked_until", time.Now().UTC().Add(-time.Minute)).Error)

	wrongLogins(t, svc, 1)
	u := reloadUser(t, db, user.ID)
	assert.Equal(t, 1, u.FailedLoginAttempts)
	assert.Nil(t, u.LockedUntil)

	wrongLogins(t, svc, MaxFailedLoginAttempts-2)
	assert.Nil(t, reloadUser(t, db, user.ID).LockedUntil)
	wrongLogins(t, svc, 1)
	assert.NotNil(t, reloadUser(t, db, user.ID).LockedUntil)
}

func TestLogin_LockTimeIsZoneIndependent(t *testing.T) {
	zones := map[string]*time.Location{
		"ahead":  time.FixedZone("ahead", 9*3600),
		"behind": time.FixedZone("behind", -8*3600),
	}
	for name, loc := range zones {
		t.Run(name+"/future lock is honored", func(t *testing.T) {
			svc, db, user := newLoginTestService(t)
			require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).
				Update("locked_until", time.Now().In(loc).Add(30*time.Minute)).Error)
			_, err := svc.Login(user.Email, loginTestPassword)
			require.ErrorIs(t, err, ErrInvalidLogin)
			wrongLogins(t, svc, 2)
			assert.Equal(t, 0, reloadUser(t, db, user.ID).FailedLoginAttempts)
		})
		t.Run(name+"/past lock is expired", func(t *testing.T) {
			svc, db, user := newLoginTestService(t)
			require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]any{
				"locked_until":          time.Now().In(loc).Add(-30 * time.Minute),
				"failed_login_attempts": MaxFailedLoginAttempts,
			}).Error)
			wrongLogins(t, svc, 1)
			u := reloadUser(t, db, user.ID)
			assert.Equal(t, 1, u.FailedLoginAttempts)
			assert.Nil(t, u.LockedUntil)
			token, err := svc.Login(user.Email, loginTestPassword)
			require.NoError(t, err)
			assert.NotEmpty(t, token)
		})
	}
}

func TestLogin_LockIsStoredInUTC(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	wrongLogins(t, svc, MaxFailedLoginAttempts)
	var raw string
	require.NoError(t, db.Raw("SELECT locked_until FROM users WHERE id = ?", user.ID).Scan(&raw).Error)
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	require.NoError(t, err, "stored value %q", raw)
	_, offset := parsed.Zone()
	assert.Zero(t, offset, "stored value %q", raw)
}

func TestLogin_SuccessResetsStateWithTargetedUpdate(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	wrongLogins(t, svc, 2)

	// Concurrent changes to other columns must survive a successful sign-in.
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).
		Update("name", "Renamed").Error)

	token, err := svc.Login(user.Email, loginTestPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	u := reloadUser(t, db, user.ID)
	assert.Equal(t, 0, u.FailedLoginAttempts)
	assert.Nil(t, u.LockedUntil)
	assert.NotNil(t, u.LastLogin)
	assert.Equal(t, "Renamed", u.Name)
}

func TestLogin_DisabledDuringSignInIsRefused(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).Update("enabled", false).Error)

	// Wrong-password failure update must not touch a disabled account.
	rows, err := svc.recordFailedLogin(user.ID, time.Now())
	require.NoError(t, err)
	assert.Zero(t, rows)
	assert.Equal(t, 0, reloadUser(t, db, user.ID).FailedLoginAttempts)
}

func TestLogin_DatabaseErrorIsReportedAsUnavailable(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = svc.Login(user.Email, loginTestPassword)
	require.ErrorIs(t, err, ErrLoginUnavailable)
}

func TestLogin_FailureRecordingErrorIsReportedAsUnavailable(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	// Break only the write path: drop the counter column.
	require.NoError(t, db.Exec("ALTER TABLE users DROP COLUMN failed_login_attempts").Error)

	_, err := svc.Login(user.Email, "wrong-password")
	require.ErrorIs(t, err, ErrLoginUnavailable)
}

func TestLogin_SuccessRecordingErrorIsReportedAsUnavailable(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	require.NoError(t, db.Exec("ALTER TABLE users DROP COLUMN last_login").Error)

	_, err := svc.Login(user.Email, loginTestPassword)
	require.ErrorIs(t, err, ErrLoginUnavailable)
}

func TestLogin_ReturningSupportedBySQLite(t *testing.T) {
	db := setupLoginTestDB(t)
	var version string
	require.NoError(t, db.Raw("SELECT sqlite_version()").Scan(&version).Error)
	t.Logf("sqlite %s", version)

	user := models.User{UUID: "u-1", Email: "r@example.com", Enabled: true}
	require.NoError(t, db.Create(&user).Error)
	var out failedLoginState
	res := db.Raw("UPDATE users SET failed_login_attempts = failed_login_attempts + 1 WHERE id = ? RETURNING failed_login_attempts, locked_until", user.ID).Scan(&out)
	require.NoError(t, res.Error)
	assert.Equal(t, int64(1), res.RowsAffected)
	assert.Equal(t, 1, out.FailedLoginAttempts)
}

func TestLogin_ConcurrentFailuresAreCountedAtomically(t *testing.T) {
	svc, db, user := newLoginTestService(t)

	const workers = 24
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := svc.Login(user.Email, "wrong-password")
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.ErrorIs(t, err, ErrInvalidLogin)
	}

	u := reloadUser(t, db, user.ID)
	// Attempts that raced past the lock check may not all be recorded, but the
	// counter never exceeds the threshold and the account ends up locked.
	assert.Equal(t, MaxFailedLoginAttempts, u.FailedLoginAttempts)
	require.NotNil(t, u.LockedUntil)
	assert.True(t, u.LockedUntil.After(time.Now()))
}

func TestChangePassword_EndsOtherSessionsAndClearsLock(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	oldToken, err := svc.Login(user.Email, loginTestPassword)
	require.NoError(t, err)
	wrongLogins(t, svc, MaxFailedLoginAttempts)
	require.NoError(t, db.Model(&models.User{}).Where("id = ?", user.ID).Update("locked_until", nil).Error)
	before := reloadUser(t, db, user.ID)

	require.NoError(t, svc.ChangePassword(user.ID, loginTestPassword, "new-password-1"))

	after := reloadUser(t, db, user.ID)
	assert.Equal(t, before.SessionVersion+1, after.SessionVersion)
	assert.Equal(t, 0, after.FailedLoginAttempts)
	assert.Nil(t, after.LockedUntil)

	_, _, err = svc.AuthenticateToken(oldToken)
	assert.Error(t, err, "sessions issued before the change must end")

	token, err := svc.Login(user.Email, "new-password-1")
	require.NoError(t, err)
	_, _, err = svc.AuthenticateToken(token)
	assert.NoError(t, err)
}

func TestChangePassword_WrongCurrentPasswordChangesNothing(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	before := reloadUser(t, db, user.ID)

	require.Error(t, svc.ChangePassword(user.ID, "wrong", "new-password-1"))

	after := reloadUser(t, db, user.ID)
	assert.Equal(t, before.PasswordHash, after.PasswordHash)
	assert.Equal(t, before.SessionVersion, after.SessionVersion)
}

func TestApplyPasswordChange_UnknownUser(t *testing.T) {
	_, db, _ := newLoginTestService(t)
	require.Error(t, applyPasswordChange(db, 9999, "hash"))
}

func TestLogin_AccountDisabledDuringSignInIsRefused(t *testing.T) {
	for _, password := range []string{loginTestPassword, "wrong-password"} {
		svc, db, user := newLoginTestService(t)
		// Disable the account right after the sign-in loads it.
		require.NoError(t, db.Callback().Query().After("gorm:query").Register("test:disable", func(tx *gorm.DB) {
			_ = tx.Session(&gorm.Session{NewDB: true}).Exec("UPDATE users SET enabled = ?", false).Error
		}))

		token, err := svc.Login(user.Email, password)
		assert.Empty(t, token)
		require.ErrorIs(t, err, ErrInvalidLogin)
		require.NoError(t, db.Callback().Query().Remove("test:disable"))
	}
}

func TestChangePassword_RejectsUnhashablePassword(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	before := reloadUser(t, db, user.ID)
	tooLong := string(make([]byte, 80)) // bcrypt rejects inputs beyond 72 bytes
	require.Error(t, svc.ChangePassword(user.ID, loginTestPassword, tooLong))
	assert.Equal(t, before.PasswordHash, reloadUser(t, db, user.ID).PasswordHash)
}

func TestChangePassword_StorageFailureIsReported(t *testing.T) {
	svc, db, user := newLoginTestService(t)
	require.NoError(t, db.Exec("ALTER TABLE users DROP COLUMN session_version").Error)
	require.Error(t, svc.ChangePassword(user.ID, loginTestPassword, "new-password-1"))
}

func TestTokenForUser(t *testing.T) {
	svc, _, user := newLoginTestService(t)
	token, err := svc.TokenForUser(user.ID)
	require.NoError(t, err)
	_, _, err = svc.AuthenticateToken(token)
	require.NoError(t, err)

	_, err = svc.TokenForUser(9999)
	require.Error(t, err)
}

func TestLogin_AccountWithoutStoredPasswordNeverSignsIn(t *testing.T) {
	svc, db, _ := newLoginTestService(t)
	pending := models.User{UUID: "pending-1", Email: "pending@example.com", Enabled: true}
	require.NoError(t, db.Create(&pending).Error)

	for _, password := range []string{"", "anything", loginTestPassword} {
		token, err := svc.Login(pending.Email, password)
		assert.Empty(t, token)
		require.ErrorIs(t, err, ErrInvalidLogin)
	}
	assert.False(t, checkPasswordUniformCost(&pending, ""))
}

func TestLogin_AccountWithoutStoredPasswordRunsFullCostCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("duration comparison skipped in short mode")
	}
	svc, db, user := newLoginTestService(t)
	pending := models.User{UUID: "pending-2", Email: "pending2@example.com", Enabled: true}
	require.NoError(t, db.Create(&pending).Error)
	placeholderHash()

	median := func(email string) time.Duration {
		const n = 5
		d := make([]time.Duration, 0, n)
		for i := 0; i < n; i++ {
			start := time.Now()
			_, _ = svc.Login(email, "x")
			d = append(d, time.Since(start))
		}
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		return d[n/2]
	}
	stored := median(user.Email)
	empty := median(pending.Email)
	assert.GreaterOrEqual(t, float64(empty), 0.5*float64(stored), "empty=%v stored=%v", empty, stored)
}
