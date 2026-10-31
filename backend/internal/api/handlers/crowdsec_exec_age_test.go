package handlers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// statLine builds a /proc/<pid>/stat line with the given comm and start time in clock ticks. Fields 4 to 21 are
// filled with zeros, so starttime lands at field 22 as the kernel lays it out.
func statLine(comm string, startTicks uint64) []byte {
	return []byte(fmt.Sprintf("1234 (%s) S%s %d\n", comm, strings.Repeat(" 0", 18), startTicks))
}

func TestProcClockTicksPerSecondIsUserHZ(t *testing.T) {
	t.Parallel()
	// Linux exposes starttime in USER_HZ ticks, which is 100 on every platform Charon ships. A change here must be
	// deliberate, because every adopted-engine age is computed from it.
	assert.Equal(t, 100, procClockTicksPerSecond)
}

func TestParseProcessAgeUsesClockTicks(t *testing.T) {
	t.Parallel()
	// 100 ticks is one second after boot; 1000 s of uptime leaves 999 s of age.
	age, ok := parseProcessAge(statLine("crowdsec", 100), []byte("1000.00 500.00\n"))
	require.True(t, ok)
	assert.Equal(t, 999*time.Second, age)
}

func TestParseProcessAgeCommWithSpacesAndParentheses(t *testing.T) {
	t.Parallel()
	for _, comm := range []string{"crowd sec", "crowd (sec)", "x) y (z", "a)b)c"} {
		t.Run(comm, func(t *testing.T) {
			t.Parallel()
			age, ok := parseProcessAge(statLine(comm, 300), []byte("1000.00 0.00\n"))
			require.True(t, ok, "comm %q must not shift the fields", comm)
			assert.Equal(t, 997*time.Second, age)
		})
	}
}

func TestParseProcessAgeRejectsImplausibleAges(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		stat, uptime string
	}{
		// lxcfs-style: the container uptime is shorter than the host start time the stat file reports.
		"start after uptime": {string(statLine("crowdsec", 200000)), "1000.00 0.00\n"},
		"no closing paren":   {"1234 crowdsec S 1 2 3\n", "1000.00 0.00\n"},
		"too few fields":     {"1234 (crowdsec) S 1 2\n", "1000.00 0.00\n"},
		"bad start time":     {"1234 (crowdsec) S" + strings.Repeat(" 0", 18) + " soon\n", "1000.00 0.00\n"},
		"empty uptime":       {string(statLine("crowdsec", 100)), ""},
		"bad uptime":         {string(statLine("crowdsec", 100)), "later\n"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, ok := parseProcessAge([]byte(tc.stat), []byte(tc.uptime))
			assert.False(t, ok, "an implausible or unreadable age is unknown, so the engine counts as old enough")
		})
	}
}

func TestProcessAgeReadsProcFiles(t *testing.T) {
	t.Parallel()
	procDir := t.TempDir()
	writeFakeStat(t, procDir, 77, string(statLine("crowdsec", 500)))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "uptime"), []byte("900.00 1.00\n"), 0o600)) // #nosec G306 -- test fixture

	age, ok := processAge(procDir, 77)
	require.True(t, ok)
	assert.Equal(t, 895*time.Second, age)

	_, ok = processAge(procDir, 78)
	assert.False(t, ok, "a pid with no stat file is unknown")

	require.NoError(t, os.Remove(filepath.Join(procDir, "uptime")))
	_, ok = processAge(procDir, 77)
	assert.False(t, ok, "a missing uptime file is unknown")
}

func TestEngineAgeUsesSpawnTimeForSpawnedEngines(t *testing.T) {
	t.Parallel()
	e, _, _ := newProcExecutor(t) // an empty proc tree: pid 42 has no entry, whatever the host runs
	e.recordStart(42, time.Now().Add(-3*time.Second))

	age, known := e.EngineAge(42)
	require.True(t, known)
	assert.GreaterOrEqual(t, age, 3*time.Second)
	assert.Less(t, age, time.Minute)

	e.forgetStart(42)
	_, known = e.EngineAge(42)
	assert.False(t, known, "an exited engine with no proc entry has no known age")
}

func TestEngineAgeFallsBackToProcForAdoptedEngines(t *testing.T) {
	t.Parallel()
	e, procDir, _ := newProcExecutor(t)
	writeFakeStat(t, procDir, 55, string(statLine("crowdsec", 100)))
	require.NoError(t, os.WriteFile(filepath.Join(procDir, "uptime"), []byte("20.00 0.00\n"), 0o600)) // #nosec G306 -- test fixture

	age, known := e.EngineAge(55)
	require.True(t, known)
	assert.Equal(t, 19*time.Second, age)
}

func TestDefaultCrowdsecExecutorRecordsSpawnAndClearsOnExit(t *testing.T) {
	e, bin, dir := newStubExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), eventuallyTimeout)
	defer cancel()

	pid, err := e.Start(ctx, bin, dir)
	require.NoError(t, err)

	age, known := e.EngineAge(pid)
	require.True(t, known)
	assert.Less(t, age, eventuallyTimeout)

	require.NoError(t, e.Stop(ctx, dir))
	require.Eventually(t, func() bool {
		e.startMu.Lock()
		defer e.startMu.Unlock()
		_, still := e.startedAt[pid]
		return !still
	}, eventuallyTimeout, 10*time.Millisecond, "the spawn time is dropped once the engine exits")
}
