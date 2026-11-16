package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"strings"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
)

// Errors of the management access checks. Match them with errors.Is.
var (
	// ErrUnusableClientAddress reports a caller address that cannot be judged: unparsable, zoned or unspecified. While
	// a management list is enforced such a caller is refused.
	ErrUnusableClientAddress = errors.New("client address is not usable for management access")
)

// ManagementListError names the access list whose stored rules the proxy cannot use.
type ManagementListError struct {
	List string
	Err  error
}

func (e *ManagementListError) Error() string {
	return fmt.Sprintf("access list %q cannot be used for management access: %v", e.List, e.Err)
}

func (e *ManagementListError) Unwrap() error { return e.Err }

// ManagementVerdict is the outcome of the management set for one caller.
type ManagementVerdict struct {
	// Allowed is false when a listed, enabled list refuses the caller.
	Allowed bool
	// Refused is the list that refused the caller when Allowed is false.
	Refused *models.AccessList
	// Decision is the decision of the refusing list.
	Decision Decision
	// Skipped names the members that were not applied: lists no longer present, and block-lists whose rules cannot be
	// used.
	Skipped []string
}

// ACLEnabled reports whether access control is on. The runtime switch in the settings wins; without one the static
// configuration decides. getSetting is the settings lookup (a cache in the middleware, the database elsewhere).
func ACLEnabled(getSetting func(key string) (string, bool), cfg config.SecurityConfig) bool {
	if val, ok := getSetting(models.SecurityACLEnabledSettingKey); ok {
		return strings.EqualFold(val, "true")
	}
	return cfg.ACLMode == "enabled"
}

// SettingGetter returns a settings lookup that reads the database for each key.
func SettingGetter(db *gorm.DB) func(key string) (string, bool) {
	return func(key string) (string, bool) {
		var row models.Setting
		err := db.Where("key = ?", key).First(&row).Error
		if err != nil {
			return "", false
		}
		return row.Value, true
	}
}

// ParseManagementListUUIDs decodes the stored management set, a JSON array of access-list UUIDs. Blank and duplicate
// entries are dropped. An empty value means no list is chosen.
func ParseManagementListUUIDs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}, nil
	}
	var entries []string
	if err := json.Unmarshal([]byte(raw), &entries); err != nil {
		return nil, fmt.Errorf("parse %s: %w", security.ManagementListsSettingKey, err)
	}
	seen := make(map[string]bool, len(entries))
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		uuid := strings.TrimSpace(entry)
		if uuid == "" || seen[uuid] {
			continue
		}
		seen[uuid] = true
		out = append(out, uuid)
	}
	return out, nil
}

// ManagementListUUIDs returns the stored management set. A missing row reads as an empty set.
func ManagementListUUIDs(db *gorm.DB) ([]string, error) {
	var row models.Setting
	err := db.Where("key = ?", security.ManagementListsSettingKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s setting: %w", security.ManagementListsSettingKey, err)
	}
	return ParseManagementListUUIDs(row.Value)
}

// WriteManagementLists stores the management set inside tx, as a JSON array. The list is stored sorted, so the same
// set always has the same value.
func WriteManagementLists(tx *gorm.DB, uuids []string) error {
	sorted := append([]string{}, uuids...)
	sort.Strings(sorted)
	raw, err := json.Marshal(sorted)
	if err != nil {
		return fmt.Errorf("encode %s: %w", security.ManagementListsSettingKey, err)
	}
	setting := models.Setting{Key: security.ManagementListsSettingKey, Value: string(raw), Category: "security", Type: "json"}
	err = tx.Where(models.Setting{Key: setting.Key}).Assign(setting).FirstOrCreate(&setting).Error
	if err != nil {
		return fmt.Errorf("write %s setting: %w", security.ManagementListsSettingKey, err)
	}
	return nil
}

// SeedManagementListsOnce chooses the management set for an upgrade, once: when the row is absent, the enabled lists
// that took effect on Charon's own interface under the previous check are stored. Those are the whitelists, the
// blacklists and every list with the local-network option, except geo lists with that option, whose meaning changed
// (they would now be country lists). Country lists without the option never took effect on the interface. With access
// control off the set is empty. Each geo list with the option is named in a warning and not carried over.
func SeedManagementListsOnce(db *gorm.DB, aclOn bool) error {
	var existing models.Setting
	err := db.Where("key = ?", security.ManagementListsSettingKey).First(&existing).Error
	if err == nil {
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("read %s setting: %w", security.ManagementListsSettingKey, err)
	}

	chosen := []string{}
	if aclOn {
		var lists []models.AccessList
		if err := db.Where("enabled = ?", true).Order("id").Find(&lists).Error; err != nil {
			return fmt.Errorf("read access lists for management seed: %w", err)
		}
		for _, list := range lists {
			switch {
			case strings.HasPrefix(list.Type, "geo_") && list.LocalNetworkOnly:
				logger.Log().WithField("list", list.Name).Warn("Access list with the local-network option is a country list and was not carried over to Charon's own interface; choose it again under Management access if needed")
			case strings.HasPrefix(list.Type, "geo_"):
				// A country list without the option never took effect on the interface.
			case list.Type == "whitelist" || list.Type == "blacklist" || list.LocalNetworkOnly:
				chosen = append(chosen, list.UUID)
				if list.LocalNetworkOnly {
					logger.Log().WithField("list", list.Name).Warn("Charon's own interface accepts only local addresses; behind a tunnel or proxy on this host, the visitor's address is checked")
				}
			}
		}
	}

	if err := db.Transaction(func(tx *gorm.DB) error { return WriteManagementLists(tx, chosen) }); err != nil {
		return err
	}
	logger.Log().WithField("lists", chosen).Info("Access lists protecting Charon's own interface chosen from the previous settings")
	return nil
}

// managementList returns the stored list with uuid, or override when it names that list (a proposed state under
// review).
func (s *AccessListService) managementList(uuid string, override *models.AccessList) (*models.AccessList, error) {
	if override != nil && override.UUID == uuid {
		copied := *override
		return &copied, nil
	}
	return s.GetByUUID(uuid)
}

// parseClientAddress returns the caller address the management check judges. Zoned, unparsable and unspecified
// addresses are unusable.
func parseClientAddress(raw string) (netip.Addr, error) {
	addr, err := netip.ParseAddr(strings.TrimSpace(raw))
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() {
		return netip.Addr{}, ErrUnusableClientAddress
	}
	return addr.Unmap(), nil
}

// isAllowType reports whether a list admits by an allow-rule: an allow-list, or a local-network list. A failure to use
// one of these refuses the caller; a failure to use a block-list only stops that block-list.
func isAllowType(acl *models.AccessList) bool {
	if strings.HasPrefix(acl.Type, "geo_") {
		return acl.LocalNetworkOnly
	}
	return acl.Type == "whitelist" || acl.LocalNetworkOnly
}

// EvaluateManagement judges the caller address against every listed, enabled access list, with the same decisions the
// proxy makes for a proxy host (Evaluate). Every list must allow. Lists that are not present are skipped. A stored
// list that the proxy cannot use refuses the caller when it is an allow-type list, and is skipped (and named in
// Skipped) when it is a block-list. override, when it names a listed UUID, replaces that list for this evaluation.
// An unusable caller address returns ErrUnusableClientAddress, and an allow-type list that cannot be used returns a
// *ManagementListError with the verdict refused.
func (s *AccessListService) EvaluateManagement(uuids []string, clientAddr string, admin []string, override *models.AccessList) (ManagementVerdict, error) {
	verdict := ManagementVerdict{Allowed: true}
	if len(uuids) == 0 {
		return verdict, nil
	}
	addr, err := parseClientAddress(clientAddr)
	if err != nil {
		return ManagementVerdict{}, err
	}
	for _, uuid := range uuids {
		acl, err := s.managementList(uuid, override)
		if errors.Is(err, ErrAccessListNotFound) {
			verdict.Skipped = append(verdict.Skipped, uuid)
			continue
		}
		if err != nil {
			return ManagementVerdict{}, fmt.Errorf("read management access list %s: %w", uuid, err)
		}
		decision, evalErr := s.Evaluate(acl, addr, admin)
		if evalErr != nil {
			if isAllowType(acl) {
				return ManagementVerdict{Refused: acl}, &ManagementListError{List: acl.Name, Err: evalErr}
			}
			verdict.Skipped = append(verdict.Skipped, acl.Name)
			continue
		}
		if !decision.Allowed {
			return ManagementVerdict{Refused: acl, Decision: decision, Skipped: verdict.Skipped}, nil
		}
	}
	return verdict, nil
}
