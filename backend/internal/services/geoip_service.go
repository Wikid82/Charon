// Package services provides business logic for the application.
package services

import (
	"errors"
	"net"
	"net/netip"
	"sync"

	"github.com/oschwald/geoip2-golang/v2"
)

var (
	// ErrGeoIPDatabaseNotLoaded is returned when attempting lookup without a loaded database.
	ErrGeoIPDatabaseNotLoaded = errors.New("geoip database not loaded")
	// ErrInvalidGeoIP is returned when the IP address cannot be parsed.
	ErrInvalidGeoIP = errors.New("invalid IP address")
	// ErrCountryNotFound is returned when no country code is found for the IP.
	ErrCountryNotFound = errors.New("country not found for IP")
)

// GeoIPService provides IP-to-country lookups using MaxMind GeoLite2.
type GeoIPService struct {
	mu     sync.RWMutex
	db     geoIPCountryReader
	dbPath string
}

type geoIPCountryReader interface {
	Country(ip netip.Addr) (*geoip2.Country, error)
	Close() error
}

// NewGeoIPService creates a new GeoIPService and loads the database.
// Returns an error if the database cannot be loaded.
func NewGeoIPService(dbPath string) (*GeoIPService, error) {
	svc := &GeoIPService{dbPath: dbPath}
	if err := svc.Load(); err != nil {
		return nil, err
	}
	return svc, nil
}

// Load opens or reloads the GeoIP database.
// This method is thread-safe and can be called to hot-reload the database.
func (s *GeoIPService) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close existing database if open
	if s.db != nil {
		_ = s.db.Close()
		s.db = nil
	}

	db, err := geoip2.Open(s.dbPath)
	if err != nil {
		return err
	}
	s.db = db
	return nil
}

// Close releases the database resources.
func (s *GeoIPService) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		err := s.db.Close()
		s.db = nil
		return err
	}
	return nil
}

// LookupCountry returns the ISO 3166-1 alpha-2 country code for an IP address.
// Returns ErrGeoIPDatabaseNotLoaded if database is not loaded,
// ErrInvalidGeoIP if the IP cannot be parsed, or
// ErrCountryNotFound if no country is associated with the IP.
func (s *GeoIPService) LookupCountry(ipStr string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.db == nil {
		return "", ErrGeoIPDatabaseNotLoaded
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", ErrInvalidGeoIP
	}

	// Convert net.IP to netip.Addr for v2 API
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return "", ErrInvalidGeoIP
	}

	record, err := s.db.Country(addr)
	if err != nil {
		return "", err
	}

	if record.Country.ISOCode == "" {
		return "", ErrCountryNotFound
	}

	return record.Country.ISOCode, nil
}

// IsLoaded returns true if the GeoIP database is currently loaded.
func (s *GeoIPService) IsLoaded() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.db != nil
}

// GetDatabasePath returns the configured database path.
func (s *GeoIPService) GetDatabasePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dbPath
}
