package alerts

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func fixedSource(a Alert, ok bool, err error) Source {
	return func(context.Context) (Alert, bool, error) { return a, ok, err }
}

func TestActiveEmptyWithoutSources(t *testing.T) {
	svc := NewService()

	got := svc.Active(context.Background())

	require.NotNil(t, got, "the result is never nil, so it encodes as an empty array")
	assert.Empty(t, got)
}

func TestActiveEmptyWhenNoSourceReports(t *testing.T) {
	svc := NewService(fixedSource(Alert{}, false, nil))

	got := svc.Active(context.Background())

	require.NotNil(t, got)
	assert.Empty(t, got)
}

func TestActiveSkipsFailingSource(t *testing.T) {
	failing := fixedSource(Alert{Key: "broken", Severity: models.NotificationTypeError}, true, errors.New("boom"))
	healthy := fixedSource(Alert{Key: "ok", Severity: models.NotificationTypeWarning}, true, nil)

	got := NewService(failing, healthy).Active(context.Background())

	require.Len(t, got, 1)
	assert.Equal(t, "ok", got[0].Key)
}

func TestActiveDropsDuplicateKeysKeepingFirst(t *testing.T) {
	first := Alert{Key: KeyGeoIPUnavailable, Severity: models.NotificationTypeError, AffectedListsTotal: 1}
	second := Alert{Key: KeyGeoIPUnavailable, Severity: models.NotificationTypeError, AffectedListsTotal: 9}

	got := NewService(fixedSource(first, true, nil), fixedSource(second, true, nil)).Active(context.Background())

	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0].AffectedListsTotal)
}

func TestActiveSortsBySeverityThenKey(t *testing.T) {
	sources := []Source{
		fixedSource(Alert{Key: "b-info", Severity: models.NotificationTypeInfo}, true, nil),
		fixedSource(Alert{Key: "z-error", Severity: models.NotificationTypeError}, true, nil),
		fixedSource(Alert{Key: "a-warning", Severity: models.NotificationTypeWarning}, true, nil),
		fixedSource(Alert{Key: "a-error", Severity: models.NotificationTypeError}, true, nil),
		fixedSource(Alert{Key: "x-unknown", Severity: models.NotificationType("other")}, true, nil),
		fixedSource(Alert{Key: "a-success", Severity: models.NotificationTypeSuccess}, true, nil),
	}

	got := NewService(sources...).Active(context.Background())

	keys := make([]string, 0, len(got))
	for _, a := range got {
		keys = append(keys, a.Key)
	}
	assert.Equal(t, []string{"a-error", "z-error", "a-warning", "a-success", "b-info", "x-unknown"}, keys)
}

func TestActiveCapsAffectedListsWithTotal(t *testing.T) {
	names := []string{"g", "c", "f", "a", "e", "b", "d"}
	alert := Alert{Key: KeyGeoIPUnavailable, Severity: models.NotificationTypeError, AffectedLists: names}

	got := NewService(fixedSource(alert, true, nil)).Active(context.Background())

	require.Len(t, got, 1)
	assert.Equal(t, []string{"a", "b", "c", "d", "e"}, got[0].AffectedLists)
	assert.Equal(t, 7, got[0].AffectedListsTotal)
	assert.Equal(t, []string{"g", "c", "f", "a", "e", "b", "d"}, names, "the source's slice is not reordered")
}

func TestActiveKeepsLargerSourceTotal(t *testing.T) {
	alert := Alert{Key: KeyGeoIPUnavailable, Severity: models.NotificationTypeError, AffectedLists: []string{"b", "a"}, AffectedListsTotal: 12}

	got := NewService(fixedSource(alert, true, nil)).Active(context.Background())

	require.Len(t, got, 1)
	assert.Equal(t, []string{"a", "b"}, got[0].AffectedLists)
	assert.Equal(t, 12, got[0].AffectedListsTotal)
}

func TestActiveLeavesAlertWithoutListsUntouched(t *testing.T) {
	alert := Alert{Key: "plain", Severity: models.NotificationTypeInfo}

	got := NewService(fixedSource(alert, true, nil)).Active(context.Background())

	require.Len(t, got, 1)
	assert.Nil(t, got[0].AffectedLists)
	assert.Zero(t, got[0].AffectedListsTotal)
}

func TestActivePassesDatabaseFlagThrough(t *testing.T) {
	available := false
	alert := Alert{Key: KeyGeoIPUnavailable, Severity: models.NotificationTypeError, DatabaseAvailable: &available}

	got := NewService(fixedSource(alert, true, nil)).Active(context.Background())

	require.Len(t, got, 1)
	require.NotNil(t, got[0].DatabaseAvailable)
	assert.False(t, *got[0].DatabaseAvailable)
}

func TestConstants(t *testing.T) {
	assert.Equal(t, "geoip_unavailable", KeyGeoIPUnavailable)
	assert.Equal(t, 5, MaxAffectedLists)
}
