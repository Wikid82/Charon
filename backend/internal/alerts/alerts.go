// Package alerts computes the active alerts the notification bell shows. Each Source inspects one condition at request
// time, so an alert disappears by itself once its condition stops holding.
package alerts

import (
	"context"
	"slices"
	"sort"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// KeyGeoIPUnavailable is the key of the alert that country lists are not being enforced because no GeoIP database is
// usable.
const KeyGeoIPUnavailable = "geoip_unavailable"

// MaxAffectedLists is the most list names one alert carries. The total still counts them all.
const MaxAffectedLists = 5

// Alert is one active condition shown to management users. Its JSON shape is the GET /notifications/alerts contract.
type Alert struct {
	Key                string                  `json:"key"`
	Severity           models.NotificationType `json:"severity"`
	AffectedLists      []string                `json:"affected_lists,omitempty"`
	AffectedListsTotal int                     `json:"affected_lists_total,omitempty"`
	DatabaseAvailable  *bool                   `json:"database_available,omitempty"`
}

// Source reports one alert. ok is false when its condition does not hold. A returned error skips the source for this
// request only.
type Source func(ctx context.Context) (alert Alert, ok bool, err error)

// Service evaluates its sources on every call.
type Service struct {
	sources []Source
}

// NewService returns a service that evaluates the given sources.
func NewService(sources ...Source) *Service {
	return &Service{sources: sources}
}

// Active returns the alerts whose sources report them. A failing source is logged and left out. When two sources give
// the same key, the first one in order is kept. Errors skip only their own source. The result is sorted by severity
// and then key, and is never nil.
func (s *Service) Active(ctx context.Context) []Alert {
	active := make([]Alert, 0, len(s.sources))
	seen := make(map[string]bool, len(s.sources))
	for i, source := range s.sources {
		alert, ok, err := source(ctx)
		if err != nil {
			logger.Log().WithError(err).WithField("source", i).Warn("Alert source failed; leaving its alert out")
			continue
		}
		if !ok || seen[alert.Key] {
			continue
		}
		seen[alert.Key] = true
		active = append(active, capAffectedLists(alert))
	}
	sort.SliceStable(active, func(i, j int) bool {
		if ri, rj := severityRank(active[i].Severity), severityRank(active[j].Severity); ri != rj {
			return ri < rj
		}
		return active[i].Key < active[j].Key
	})
	return active
}

// capAffectedLists sorts a copy of the alert's list names and keeps the first MaxAffectedLists. The total is raised to
// the full count when the source reported fewer.
func capAffectedLists(alert Alert) Alert {
	if len(alert.AffectedLists) == 0 {
		return alert
	}
	if alert.AffectedListsTotal < len(alert.AffectedLists) {
		alert.AffectedListsTotal = len(alert.AffectedLists)
	}
	lists := slices.Clone(alert.AffectedLists)
	slices.Sort(lists)
	if len(lists) > MaxAffectedLists {
		lists = lists[:MaxAffectedLists]
	}
	alert.AffectedLists = lists
	return alert
}

// severityRank orders alerts so that errors come first. Unknown severities come last.
func severityRank(severity models.NotificationType) int {
	switch severity {
	case models.NotificationTypeError:
		return 0
	case models.NotificationTypeWarning:
		return 1
	case models.NotificationTypeSuccess:
		return 2
	case models.NotificationTypeInfo:
		return 3
	default:
		return 4
	}
}
