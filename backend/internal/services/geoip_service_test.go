package services

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/oschwald/geoip2-golang/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeGeoIPReader struct {
	isoCode string
	err     error
}

func (f *fakeGeoIPReader) Country(_ netip.Addr) (*geoip2.Country, error) {
	if f.err != nil {
		return nil, f.err
	}
	rec := &geoip2.Country{}
	rec.Country.ISOCode = f.isoCode
	return rec, nil
}

func (f *fakeGeoIPReader) Close() error { return nil }

// TestNewGeoIPService_InvalidPath tests creation with an invalid database path.
func TestNewGeoIPService_InvalidPath(t *testing.T) {
	_, err := NewGeoIPService("/nonexistent/path/to/GeoLite2-Country.mmdb")
	assert.Error(t, err)
}

// TestGeoIPService_NotLoaded tests lookup behavior when database is not loaded.
func TestGeoIPService_NotLoaded(t *testing.T) {
	svc := &GeoIPService{dbPath: "/invalid/path.mmdb"}

	// Should return error when database not loaded
	_, err := svc.LookupCountry("8.8.8.8")
	assert.ErrorIs(t, err, ErrGeoIPDatabaseNotLoaded)

	// IsLoaded should return false
	assert.False(t, svc.IsLoaded())
}

// TestGeoIPService_InvalidIP tests lookup with invalid IP addresses.
func TestGeoIPService_InvalidIP(t *testing.T) {
	svc := &GeoIPService{dbPath: "/test/path.mmdb", db: &fakeGeoIPReader{isoCode: "US"}}
	_, err := svc.LookupCountry("not-an-ip")
	assert.ErrorIs(t, err, ErrInvalidGeoIP)
}

func TestGeoIPService_LookupCountry_CountryNotFound(t *testing.T) {
	svc := &GeoIPService{dbPath: "/test/path.mmdb", db: &fakeGeoIPReader{isoCode: ""}}
	_, err := svc.LookupCountry("8.8.8.8")
	assert.ErrorIs(t, err, ErrCountryNotFound)
}

func TestGeoIPService_LookupCountry_Success(t *testing.T) {
	svc := &GeoIPService{dbPath: "/test/path.mmdb", db: &fakeGeoIPReader{isoCode: "US"}}
	cc, err := svc.LookupCountry("8.8.8.8")
	assert.NoError(t, err)
	assert.Equal(t, "US", cc)
}

func TestGeoIPService_LookupCountry_ReaderError(t *testing.T) {
	svc := &GeoIPService{dbPath: "/test/path.mmdb", db: &fakeGeoIPReader{err: errors.New("boom")}}
	_, err := svc.LookupCountry("8.8.8.8")
	assert.Error(t, err)
}

// TestGeoIPService_Close tests closing behavior.
func TestGeoIPService_Close(t *testing.T) {
	svc := &GeoIPService{dbPath: "/test/path.mmdb"}

	// Close on nil db should not error
	err := svc.Close()
	assert.NoError(t, err)
	assert.False(t, svc.IsLoaded())
}

// TestGeoIPService_GetDatabasePath tests the path getter.
func TestGeoIPService_GetDatabasePath(t *testing.T) {
	expectedPath := "/app/data/geoip/GeoLite2-Country.mmdb"
	svc := &GeoIPService{dbPath: expectedPath}

	assert.Equal(t, expectedPath, svc.GetDatabasePath())
}

// TestGeoIPService_ConcurrentAccess tests thread-safety of lookups.
func TestGeoIPService_ConcurrentAccess(t *testing.T) {
	svc := &GeoIPService{dbPath: "/test/path.mmdb"}

	// Launch multiple goroutines to access the service concurrently
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = svc.IsLoaded()
			_, _ = svc.LookupCountry("8.8.8.8")
			_ = svc.GetDatabasePath()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

// Integration test - only runs if a real GeoIP database is available
// To run: go test -tags=integration -run TestGeoIPService_Integration
func TestGeoIPService_Integration(t *testing.T) {
	// Check common paths for GeoIP database
	homePath := filepath.Join(os.Getenv("HOME"), ".local", "share", "GeoIP", "GeoLite2-Country.mmdb")
	possiblePaths := []string{
		"/app/data/geoip/GeoLite2-Country.mmdb",
		"../../../data/geoip/GeoLite2-Country.mmdb",
		homePath,
	}

	var dbPath string
	for _, p := range possiblePaths {
		if _, err := os.Stat(p); err == nil { //nolint:gosec // G703: test path from controlled location
			dbPath = p
			break
		}
	}

	if dbPath == "" {
		t.Skip("GeoIP database not found, skipping integration test")
	}

	svc, err := NewGeoIPService(dbPath)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	t.Run("IsLoaded", func(t *testing.T) {
		assert.True(t, svc.IsLoaded())
	})

	t.Run("LookupKnownIP", func(t *testing.T) {
		// Google's DNS is in the US
		country, err := svc.LookupCountry("8.8.8.8")
		assert.NoError(t, err)
		assert.Equal(t, "US", country)
	})

	t.Run("LookupPrivateIP", func(t *testing.T) {
		// Private IPs typically return no country
		_, err := svc.LookupCountry("192.168.1.1")
		// This might return ErrCountryNotFound or a valid country depending on database
		// Either is acceptable behavior
		if err != nil {
			assert.ErrorIs(t, err, ErrCountryNotFound)
		}
	})

	t.Run("LookupInvalidIP", func(t *testing.T) {
		_, err := svc.LookupCountry("invalid-ip")
		assert.ErrorIs(t, err, ErrInvalidGeoIP)
	})

	t.Run("LookupIPv6", func(t *testing.T) {
		// Google's IPv6 DNS
		country, err := svc.LookupCountry("2001:4860:4860::8888")
		assert.NoError(t, err)
		assert.Equal(t, "US", country)
	})

	t.Run("Reload", func(t *testing.T) {
		// Test hot-reload capability
		err := svc.Load()
		assert.NoError(t, err)
		assert.True(t, svc.IsLoaded())

		// Verify lookup still works after reload
		country, err := svc.LookupCountry("8.8.8.8")
		assert.NoError(t, err)
		assert.Equal(t, "US", country)
	})
}

// TestGeoIPService_ErrorTypes verifies error constants are properly defined.
func TestGeoIPService_ErrorTypes(t *testing.T) {
	assert.NotNil(t, ErrGeoIPDatabaseNotLoaded)
	assert.NotNil(t, ErrInvalidGeoIP)
	assert.NotNil(t, ErrCountryNotFound)

	// Verify error messages
	assert.Contains(t, ErrGeoIPDatabaseNotLoaded.Error(), "database")
	assert.Contains(t, ErrInvalidGeoIP.Error(), "invalid")
	assert.Contains(t, ErrCountryNotFound.Error(), "country")
}
