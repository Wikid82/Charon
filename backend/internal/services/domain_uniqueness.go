package services

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// errDomainExists is returned when a domain is already claimed by another
// row of the same resource type.
var errDomainExists = errors.New("domain already exists")

// checkCrossTableDomainConflict returns an error if any comma-separated domain in
// domainNames is already claimed by a row in the *other* resource table
// (ProxyHost when checking from RedirectionHost, and vice versa). Same-table
// checks are handled by checkSameTableDomainConflict. Comparison is
// per-individual domain (both sides' comma-separated lists are split,
// trimmed and lowercased).
//
// otherTable is the GORM model of the *other* resource, e.g. a
// RedirectionHost service passes &models.ProxyHost{} and vice versa.
//
// If otherTable's underlying table does not exist (e.g. a test DB that only
// migrates one of the two models), no conflict is reported — there is
// nothing to check against.
func checkCrossTableDomainConflict(db *gorm.DB, domainNames string, otherTable any) error {
	if !db.Migrator().HasTable(otherTable) {
		return nil
	}

	conflict, err := domainsOverlap(db, domainNames, otherTable, 0)
	if err != nil {
		return fmt.Errorf("checking cross-table domain conflict: %w", err)
	}
	if conflict {
		return errors.New("domain already in use by another host")
	}
	return nil
}

// checkSameTableDomainConflict returns errDomainExists if any individual
// domain in domainNames is already claimed by another row of table. The row
// whose ID equals excludeID (when > 0) is ignored so updates do not collide
// with themselves.
func checkSameTableDomainConflict(db *gorm.DB, domainNames string, table any, excludeID uint) error {
	conflict, err := domainsOverlap(db, domainNames, table, excludeID)
	if err != nil {
		return fmt.Errorf("checking domain uniqueness: %w", err)
	}
	if conflict {
		return errDomainExists
	}
	return nil
}

// domainsOverlap reports whether any domain in domainNames appears in any
// row of table (excluding excludeID when > 0).
//
// Matching is deliberately limited to exact, case-insensitive host names.
// Wildcard-vs-specific overlap (e.g. *.example.com vs www.example.com),
// trailing-dot forms and IDN/punycode equivalence are intentional non-goals:
// Caddy gives specific hosts precedence over wildcards, so those pairs can
// coexist, and names are compared exactly as entered.
func domainsOverlap(db *gorm.DB, domainNames string, table any, excludeID uint) (bool, error) {
	candidates := splitDomains(domainNames)
	if len(candidates) == 0 {
		return false, nil
	}

	query := db.Model(table).Select("domain_names")
	if excludeID > 0 {
		query = query.Where("id != ?", excludeID)
	}

	var rows []struct {
		DomainNames string
	}
	if err := query.Find(&rows).Error; err != nil {
		return false, err
	}

	for _, row := range rows {
		for existing := range splitDomains(row.DomainNames) {
			if candidates[existing] {
				return true, nil
			}
		}
	}
	return false, nil
}

// splitDomains parses a comma-separated domain list into a lowercased,
// trimmed set for per-domain comparison. Empty entries are dropped.
func splitDomains(domainNames string) map[string]bool {
	parts := strings.Split(domainNames, ",")
	set := make(map[string]bool, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		set[p] = true
	}
	return set
}

// CanonicalDomainNames returns domainNames with each element trimmed and
// lowercased, empty elements dropped and duplicates removed (first
// occurrence wins), rejoined with ",".
func CanonicalDomainNames(domainNames string) string {
	parts := strings.Split(domainNames, ",")
	seen := make(map[string]bool, len(parts))
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return strings.Join(out, ",")
}

// DomainsShareAny reports whether two comma-separated domain lists have at
// least one domain in common after canonicalization.
func DomainsShareAny(a, b string) bool {
	set := splitDomains(a)
	for d := range splitDomains(b) {
		if set[d] {
			return true
		}
	}
	return false
}
