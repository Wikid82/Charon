package services

import (
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// CheckDomainConflict returns an error if any comma-separated domain in
// domainNames is already claimed by a row in the *other* resource table
// (ProxyHost when checking from RedirectionHost, and vice versa). It never
// queries the caller's own table — each service's existing same-table check
// (unchanged) already covers that. Comparison is per-individual domain (both
// sides' comma-separated lists are split and lowercased), since the two
// tables have no shared string convention to exact-match against.
//
// otherTable is the GORM model of the *other* resource, e.g. a
// RedirectionHost service passes &models.ProxyHost{} and vice versa.
//
// This is purely additive: it only ever adds a new rejection reason (a
// domain claimed by the other resource type). It never relaxes, replaces, or
// bypasses either service's existing same-table uniqueness check. If
// otherTable's underlying table does not exist (e.g. a test DB that only
// migrates one of the two models), no conflict is reported — there is
// nothing to check against, and this must never turn into a hard error for
// callers that legitimately don't know about the other resource type.
func CheckDomainConflict(db *gorm.DB, domainNames string, otherTable any) error {
	candidates := splitDomains(domainNames)
	if len(candidates) == 0 {
		return nil
	}

	if !db.Migrator().HasTable(otherTable) {
		return nil
	}

	var rows []struct {
		DomainNames string
	}
	if err := db.Model(otherTable).Select("domain_names").Find(&rows).Error; err != nil {
		return fmt.Errorf("checking cross-table domain conflict: %w", err)
	}

	for _, row := range rows {
		for existing := range splitDomains(row.DomainNames) {
			if candidates[existing] {
				return errors.New("domain already in use by another host")
			}
		}
	}

	return nil
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
