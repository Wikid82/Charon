package services

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// setupConcurrentHostsDB opens a file-backed, multi-connection WAL database so
// concurrent writers genuinely race on separate connections.
func setupConcurrentHostsDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(15000)&_pragma=journal_mode(WAL)", filepath.Join(t.TempDir(), "hosts.db"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.RedirectionHost{}))
	return db
}

// runConcurrently starts every fn at the same instant and returns the number
// that returned nil, failing the test on any error that is not a domain
// conflict.
func runConcurrently(t *testing.T, fns []func() error) int {
	t.Helper()
	start := make(chan struct{})
	errs := make(chan error, len(fns))
	var wg sync.WaitGroup
	for _, fn := range fns {
		wg.Add(1)
		go func(fn func() error) {
			defer wg.Done()
			<-start
			errs <- fn()
		}(fn)
	}
	close(start)
	wg.Wait()
	close(errs)

	succeeded := 0
	for err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, errDomainExists), err.Error() == "domain already in use by another host":
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	return succeeded
}

func TestProxyHostService_Create_ConcurrentOverlappingDomainsSingleWinner(t *testing.T) {
	db := setupConcurrentHostsDB(t)
	svc := NewProxyHostService(db)

	const n = 12
	fns := make([]func() error, n)
	for i := range fns {
		i := i
		fns[i] = func() error {
			return svc.Create(&models.ProxyHost{
				UUID:        fmt.Sprintf("ph-%d", i),
				DomainNames: fmt.Sprintf("shared.example.com,own%d.example.com", i),
				ForwardHost: "127.0.0.1",
				ForwardPort: 8080,
			})
		}
	}

	assert.Equal(t, 1, runConcurrently(t, fns))
	var count int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestHostServices_Create_ConcurrentCrossTableSingleWinner(t *testing.T) {
	db := setupConcurrentHostsDB(t)
	ph := NewProxyHostService(db)
	rh := NewRedirectionHostService(db)

	const pairs = 6
	var fns []func() error
	for i := 0; i < pairs; i++ {
		i := i
		fns = append(fns,
			func() error {
				return ph.Create(&models.ProxyHost{
					UUID:        fmt.Sprintf("ph-%d", i),
					DomainNames: "cross.example.com",
					ForwardHost: "127.0.0.1",
					ForwardPort: 8080,
				})
			},
			func() error {
				return rh.Create(&models.RedirectionHost{
					UUID:        fmt.Sprintf("rh-%d", i),
					DomainNames: "Cross.Example.com",
					TargetURL:   "https://elsewhere.example.org",
					StatusCode:  301,
				})
			},
		)
	}

	assert.Equal(t, 1, runConcurrently(t, fns))
	var phCount, rhCount int64
	require.NoError(t, db.Model(&models.ProxyHost{}).Count(&phCount).Error)
	require.NoError(t, db.Model(&models.RedirectionHost{}).Count(&rhCount).Error)
	assert.Equal(t, int64(1), phCount+rhCount)
}

func TestHostServices_Update_ConcurrentOverlappingDomainsSingleWinner(t *testing.T) {
	db := setupConcurrentHostsDB(t)
	ph := NewProxyHostService(db)
	rh := NewRedirectionHostService(db)

	const n = 6
	proxies := make([]*models.ProxyHost, n)
	redirects := make([]*models.RedirectionHost, n)
	for i := 0; i < n; i++ {
		proxies[i] = &models.ProxyHost{UUID: fmt.Sprintf("ph-%d", i), DomainNames: fmt.Sprintf("p%d.example.com", i), ForwardHost: "127.0.0.1", ForwardPort: 8080}
		require.NoError(t, ph.Create(proxies[i]))
		redirects[i] = &models.RedirectionHost{UUID: fmt.Sprintf("rh-%d", i), DomainNames: fmt.Sprintf("r%d.example.com", i), TargetURL: "https://elsewhere.example.org", StatusCode: 301}
		require.NoError(t, rh.Create(redirects[i]))
	}

	var fns []func() error
	for i := 0; i < n; i++ {
		p, r := proxies[i], redirects[i]
		fns = append(fns,
			func() error { p.DomainNames = "contested.example.com"; return ph.Update(p) },
			func() error { r.DomainNames = "contested.example.com"; return rh.Update(r) },
		)
	}

	assert.Equal(t, 1, runConcurrently(t, fns))
}

func TestWithWriteLock_PropagatesCallbackAndLockErrors(t *testing.T) {
	db := setupConcurrentHostsDB(t)

	sentinel := errors.New("callback failed")
	err := WithWriteLock(db, &models.ProxyHost{}, func(*gorm.DB) error { return sentinel })
	require.ErrorIs(t, err, sentinel)

	// A model whose table does not exist makes the lock statement fail.
	type missingTable struct{ ID uint }
	called := false
	err = WithWriteLock(db, &missingTable{}, func(*gorm.DB) error { called = true; return nil })
	require.Error(t, err)
	assert.Contains(t, err.Error(), "acquire write lock")
	assert.False(t, called)
}
