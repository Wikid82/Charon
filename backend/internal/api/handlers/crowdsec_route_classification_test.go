package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// routeClass says how a mutating /admin/crowdsec route relates to a background hub install (#1615).
type routeClass string

const (
	// routeBusyChecked routes change DataDir (or the engine's process) and must answer 409 crowdsec_busy
	// while an install job runs.
	routeBusyChecked routeClass = "busy-checked"
	// routeLAPIOnly routes only talk to the local API or the engine database, which snapshots never touch.
	routeLAPIOnly routeClass = "LAPI-only"
	// routeOutsideDataDir routes write nothing a snapshot restore could revert.
	routeOutsideDataDir routeClass = "outside DataDir"
)

type routeClassification struct {
	class  routeClass
	reason string
}

// mutatingCrowdsecRoutes classifies every POST, PUT and DELETE route registered under /admin/crowdsec.
// TestEveryMutatingCrowdsecRouteIsClassified fails when a route is added, renamed or removed without a row
// here, so the decision "does a running install have to block this?" is made for each new route.
var mutatingCrowdsecRoutes = map[string]routeClassification{
	"POST /admin/crowdsec/start": {routeBusyChecked,
		"starting the engine writes the bouncer key file and the PID file and would race the install; the check must run before the executor call"},
	"POST /admin/crowdsec/stop": {routeBusyChecked,
		"stopping the engine mid-install would drop the final reload; the check must run before the executor call"},
	"POST /admin/crowdsec/import": {routeBusyChecked,
		"replaces the whole configuration tree"},
	"POST /admin/crowdsec/hub-items/restore": {routeBusyChecked,
		"installs hub items; has its own restore.running flag (not lockData), so it needs its own check at the top and per section"},
	"POST /admin/crowdsec/file": {routeBusyChecked,
		"writes a file below DataDir, which a rollback restore would silently revert"},
	"POST /admin/crowdsec/presets/pull": {routeOutsideDataDir,
		"writes only hub_cache, which is engine-owned (IsEngineOwnedPath) and never snapshotted or restored; Apply tolerates a newer entry via EvictIfKey"},
	"POST /admin/crowdsec/presets/apply": {routeBusyChecked,
		"synchronous and curated apply mutate the hub tree; the async variant starts the job and answers 409 with the active job id"},
	"DELETE /admin/crowdsec/presets/jobs/:id": {routeOutsideDataDir,
		"asks the running install job to stop; the job itself rolls back from its snapshot, and the request must work while the job holds the busy flag"},
	"POST /admin/crowdsec/console/enroll": {routeBusyChecked,
		"cscli console enrol (and capi register) write online_api_credentials.yaml below DataDir/config, which a rollback restore would revert"},
	"DELETE /admin/crowdsec/console/enrollment": {routeOutsideDataDir,
		"deletes the enrollment row from Charon's own database; no file is touched"},
	"POST /admin/crowdsec/ban": {routeLAPIOnly,
		"cscli decisions add: local API and engine database only"},
	"DELETE /admin/crowdsec/ban/*value": {routeLAPIOnly,
		"cscli decisions delete: local API and engine database only"},
	"POST /admin/crowdsec/bouncer/register": {routeLAPIOnly,
		"runs register_bouncer.sh, which adds the bouncer in the engine database and writes its key to /etc/crowdsec/bouncers, outside DataDir (the DataDir bouncer_key file is written by Start, which is busy-checked)"},
	"PUT /admin/crowdsec/acquisition": {routeOutsideDataDir,
		"writes /etc/crowdsec/acquis.yaml, outside DataDir; serialised by its own leaf lock acquisMu, which nothing else is taken under; not busy-checked"},
	"POST /admin/crowdsec/whitelist": {routeBusyChecked,
		"WriteYAML writes DataDir/config/parsers/s02-enrich/charon-whitelist.yaml (inside the snapshot) and then sends SIGHUP; a rollback would revert the file while the database row stays"},
	"DELETE /admin/crowdsec/whitelist/:uuid": {routeBusyChecked,
		"same as whitelist add: rewrites or removes the whitelist YAML and reloads"},
}

// registeredMutatingCrowdsecRoutes lists "METHOD path" (path relative to the API group) for every POST, PUT and
// DELETE route RegisterRoutes adds under /admin/crowdsec.
func registeredMutatingCrowdsecRoutes(t *testing.T) []string {
	t.Helper()
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", t.TempDir())
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	var out []string
	for _, ri := range r.Routes() {
		path := strings.TrimPrefix(ri.Path, "/api/v1")
		if !strings.HasPrefix(path, "/admin/crowdsec") {
			continue
		}
		switch ri.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			out = append(out, ri.Method+" "+path)
		}
	}
	sort.Strings(out)
	return out
}

func TestEveryMutatingCrowdsecRouteIsClassified(t *testing.T) {
	t.Parallel()
	registered := registeredMutatingCrowdsecRoutes(t)
	require.NotEmpty(t, registered)

	var unclassified []string
	seen := map[string]bool{}
	for _, route := range registered {
		seen[route] = true
		if _, ok := mutatingCrowdsecRoutes[route]; !ok {
			unclassified = append(unclassified, route)
		}
	}
	require.Empty(t, unclassified, "classify these routes in mutatingCrowdsecRoutes (busy-checked, LAPI-only or outside DataDir) and, if busy-checked, add them to the busy test table")

	var stale []string
	for route := range mutatingCrowdsecRoutes {
		if !seen[route] {
			stale = append(stale, route)
		}
	}
	require.Empty(t, stale, "these classified routes are no longer registered")
}

func TestRouteClassificationsHaveAReason(t *testing.T) {
	t.Parallel()
	for route, c := range mutatingCrowdsecRoutes {
		require.Contains(t, []routeClass{routeBusyChecked, routeLAPIOnly, routeOutsideDataDir}, c.class, route)
		require.NotEmpty(t, strings.TrimSpace(c.reason), fmt.Sprintf("%s needs a reason", route))
	}
}
