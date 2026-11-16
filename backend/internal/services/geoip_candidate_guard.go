package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/oschwald/geoip2-golang/v2"
	"gorm.io/gorm"
)

// ErrGeoIPWouldLockOut refuses a GeoIP database change that would refuse the calling administrator from Charon's own
// interface under the country lists chosen to protect it. The live database and file are left as they were.
var ErrGeoIPWouldLockOut = errors.New("the GeoIP database change would lock the caller out of Charon's own interface")

type candidateGuardKey struct{}

// WithCandidateGuard returns a context carrying check. A database change run with this context calls check with the
// path of the candidate database before the live one is replaced, and is refused when check returns an error.
func WithCandidateGuard(ctx context.Context, check func(path string) error) context.Context {
	return context.WithValue(ctx, candidateGuardKey{}, check)
}

// runCandidateGuard runs the candidate check carried by ctx, if any, against path.
func runCandidateGuard(ctx context.Context, path string) error {
	check, ok := ctx.Value(candidateGuardKey{}).(func(path string) error)
	if !ok || check == nil {
		return nil
	}
	return check(path)
}

// ManagementCandidateCheck returns the check a GeoIP change runs against its candidate database. It evaluates the
// management set (chosen lists, with access control on) for the caller address with that database, the way the live
// middleware would after the change. A refusal, or a caller address that cannot be judged, returns ErrGeoIPWouldLockOut.
func ManagementCandidateCheck(db *gorm.DB, uuids []string, clientAddr string, admin []string, aclOn bool) func(path string) error {
	return func(path string) error {
		if !aclOn || len(uuids) == 0 {
			return nil
		}
		candidate, err := openCandidateLookup(path)
		if err != nil {
			return fmt.Errorf("open candidate GeoIP database: %w", err)
		}
		defer func() { _ = candidate.db.Close() }()
		svc := NewAccessListService(db)
		svc.lookup = candidate
		verdict, err := svc.EvaluateManagement(uuids, clientAddr, admin, nil)
		if err != nil || !verdict.Allowed {
			return ErrGeoIPWouldLockOut
		}
		return nil
	}
}

// candidateCountryLookup answers country lookups from a database that is not live yet.
type candidateCountryLookup struct {
	db geoIPCountryReader
}

// Available reports that the candidate can serve lookups: it was opened and validated.
func (c candidateCountryLookup) Available() bool { return c.db != nil }

// LookupCountry returns the ISO country code of ip, as GeoIPService.LookupCountry does.
func (c candidateCountryLookup) LookupCountry(ip string) (string, error) {
	return lookupISOCountry(c.db, ip)
}

// openCandidateLookup opens the database at path read-only, for a check that must not swap it in.
func openCandidateLookup(path string) (candidateCountryLookup, error) {
	db, err := geoip2.Open(path)
	if err != nil {
		return candidateCountryLookup{}, fmt.Errorf("open geoip database %s: %w", path, err)
	}
	return candidateCountryLookup{db: db}, nil
}

// lookupISOCountry returns the ISO country code of the address in ipStr, using the country database db.
func lookupISOCountry(db geoIPCountryReader, ipStr string) (string, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "", ErrInvalidGeoIP
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return "", ErrInvalidGeoIP
	}
	record, err := db.Country(addr)
	if err != nil {
		return "", err
	}
	if record.Country.ISOCode == "" {
		return "", ErrCountryNotFound
	}
	return record.Country.ISOCode, nil
}
