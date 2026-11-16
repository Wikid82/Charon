package services

import (
	"errors"
	"testing"

	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestMonitorDestination_Defaults(t *testing.T) {
	t.Parallel()
	cases := []struct {
		typ, url string
		host     string
		port     int
	}{
		{"tcp", "tcp://192.168.1.1:5432", "192.168.1.1", 5432},
		{"http", "http://10.0.0.5/status", "10.0.0.5", 80},
		{"https", "https://[fd12::5]/", "fd12::5", 443},
		{"http", "http://localhost:2019/", "localhost", 2019},
	}
	for _, tc := range cases {
		host, port, err := monitorDestination(tc.typ, tc.url)
		require.NoError(t, err, tc.url)
		assert.Equal(t, tc.host, host, tc.url)
		assert.Equal(t, tc.port, port, tc.url)
	}
}

func TestMonitorDestination_InvalidIsInputError(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ typ, url string }{
		{"tcp", "tcp://no-port"},
		{"tcp", "tcp://10.0.0.1:0"},
		{"tcp", "tcp://10.0.0.1:99999"},
		{"http", "http://10.0.0.1:abc/"},
		{"http", "http://10.0.0.1:70000/"},
		{"http", "http://[::1/"},
	} {
		_, _, err := monitorDestination(tc.typ, tc.url)
		assert.True(t, IsMonitorInputError(err), "%s %s", tc.typ, tc.url)
	}
	assert.False(t, IsMonitorInputError(errors.New("plain")))
}

func TestCheckMonitorDestination_OtherTypesUnchecked(t *testing.T) {
	t.Parallel()
	assert.NoError(t, checkMonitorDestination(nil, "orthrus", "agent-uuid"))
}

func TestCreateMonitor_RefusesDestinationsTheProbeWillNotCheck(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	resetEmergency(t)
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, nil)

	refused := []struct{ typ, url string }{
		{"tcp", "tcp://169.254.169.254:80"},
		{"http", "http://169.254.169.254/latest/meta-data/"},
		{"https", "https://[fd00:ec2::254]/"},
		{"http", "http://localhost:2019/"},
	}
	for _, tc := range refused {
		_, err := svc.CreateMonitor("refused", tc.url, tc.typ, 60, 1)
		assert.ErrorIs(t, err, ErrDestinationRefused, tc.url)
	}

	m, err := svc.CreateMonitor("lan", "tcp://192.168.1.1:5432", "tcp", 60, 1)
	require.NoError(t, err, "private addresses stay allowed")
	assert.NotZero(t, m.ID)

	_, err = svc.CreateMonitor("bad", "http://", "http", 60, 1)
	assert.True(t, IsMonitorInputError(err))
	_, err = svc.CreateMonitor("bad", "::bad", "http", 60, 1)
	assert.True(t, IsMonitorInputError(err))
	_, err = svc.CreateMonitor("bad", "tcp://10.0.0.1", "tcp", 60, 1)
	assert.True(t, IsMonitorInputError(err))
	_, err = svc.CreateMonitor("bad", "http://10.0.0.1:99999/", "http", 60, 1)
	assert.True(t, IsMonitorInputError(err))
}

func TestCreateMonitor_DatabaseErrorIsNotInputError(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	resetEmergency(t)
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, nil)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = svc.CreateMonitor("db", "tcp://10.0.0.1:80", "tcp", 60, 1)
	require.Error(t, err)
	assert.False(t, IsMonitorInputError(err))
	assert.False(t, errors.Is(err, ErrDestinationRefused))
}

func TestReportRefusedMonitors_WarnsOncePerRefusedMonitor(t *testing.T) {
	setHostSideAddrs(t)
	isolateGatewayAddrs(t, nil)
	resetEmergency(t)
	useListenerSource(t, fakeListenerSource{})
	SetEmergencyListener(config.EmergencyConfig{})
	db := setupUptimeTestDB(t)
	svc := NewUptimeService(db, nil)

	seed := []models.UptimeMonitor{
		{Name: "link-local", Type: "tcp", URL: "tcp://169.254.169.254:80", Enabled: true},
		{Name: "admin-port", Type: "http", URL: "http://localhost:2019/", Enabled: true},
		{Name: "lan", Type: "tcp", URL: "tcp://192.168.1.1:5432", Enabled: true},
		{Name: "remote-name", Type: "http", URL: "https://example.com/", Enabled: true},
		{Name: "disabled", Type: "tcp", URL: "tcp://169.254.169.254:80", Enabled: false},
	}
	for i := range seed {
		require.NoError(t, db.Create(&seed[i]).Error)
	}
	// Create does not run the check for stored rows; disabled rows are skipped.
	hook := logtest.NewLocal(logger.Log().Logger)
	defer hook.Reset()

	svc.ReportRefusedMonitors()

	var names []string
	for _, e := range hook.AllEntries() {
		if e.Message == "" {
			continue
		}
		if name, ok := e.Data["monitor_name"]; ok {
			names = append(names, name.(string))
		}
	}
	assert.ElementsMatch(t, []string{"link-local", "admin-port"}, names)
}

func TestReportRefusedMonitors_NilSafe(t *testing.T) {
	t.Parallel()
	var svc *UptimeService
	svc.ReportRefusedMonitors()
	(&UptimeService{}).ReportRefusedMonitors()
}
