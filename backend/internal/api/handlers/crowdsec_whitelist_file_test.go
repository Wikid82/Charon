package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

const managedWhitelistRel = "config/parsers/s02-enrich/charon-whitelist.yaml"

// seedWhitelistEntry adds one allow-list entry to the database.
func seedWhitelistEntry(t *testing.T, db *gorm.DB, ip string) {
	t.Helper()
	require.NoError(t, db.Create(&models.CrowdSecWhitelist{UUID: uuid.New().String(), IPOrCIDR: ip, Reason: "test"}).Error)
}

// whitelistDBHandler builds a handler on a database that holds the whitelist table.
func whitelistDBHandler(t *testing.T, eng CrowdsecExecutor, dataDir string) (*CrowdsecHandler, *gorm.DB) {
	t.Helper()
	db := setupCrowdDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdSecWhitelist{}))
	return newTestCrowdsecHandler(t, db, eng, "/bin/false", dataDir), db
}

func TestCrowdsecFiles_ManagedWhitelistRefusedOnWrite(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)
	path := f.put(managedWhitelistRel, "name: charon-whitelist\n")

	for _, spelled := range []string{managedWhitelistRel, "./" + managedWhitelistRel, "config/parsers/s02-enrich/../s02-enrich/charon-whitelist.yaml"} {
		w := f.write(spelled, "name: edited\n")
		require.Equal(t, http.StatusBadRequest, w.Code, spelled)
		var body map[string]string
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Equal(t, "file_managed_by_charon", body["error_code"], spelled)
		require.Equal(t, "Charon manages this file. Change the allow list on the Whitelist tab.", body["error"], spelled)
	}

	b, err := os.ReadFile(path) //nolint:gosec // G304: test fixture path
	require.NoError(t, err)
	require.Equal(t, "name: charon-whitelist\n", string(b), "a refused save must not modify the file")

	w := f.read(managedWhitelistRel)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestCrowdsecFiles_OtherParserFilesStayWritable(t *testing.T) {
	t.Parallel()
	f := newFilesFixture(t)

	w := f.write("config/parsers/s02-enrich/other.yaml", "name: other\nwhitelist:\n  reason: operator\n")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// whitelistSeenByEngine records the whitelist file's content at the moment the engine starts.
type whitelistSeenByEngine struct {
	*recordingEngine
	path string
	mu   sync.Mutex
	seen []string
}

func (e *whitelistSeenByEngine) Start(ctx context.Context, bin, dir string) (int, error) {
	content, err := os.ReadFile(e.path) //nolint:gosec // G304: test fixture path
	e.mu.Lock()
	if err != nil {
		e.seen = append(e.seen, "missing")
	} else {
		e.seen = append(e.seen, string(content))
	}
	e.mu.Unlock()
	return e.recordingEngine.Start(ctx, bin, dir)
}

func TestCrowdsecLifecycle_StartWritesWhitelistFromDBBeforeEngineStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	eng := &whitelistSeenByEngine{recordingEngine: &recordingEngine{}, path: filepath.Join(dir, managedWhitelistRel)}
	h, db := whitelistDBHandler(t, eng, dir)
	seedWhitelistEntry(t, db, "10.0.0.5")
	require.NoError(t, os.MkdirAll(filepath.Dir(eng.path), 0o750))
	require.NoError(t, os.WriteFile(eng.path, []byte("stale archive copy\n"), 0o600))

	res, err := h.StartCrowdSec(context.Background())

	require.NoError(t, err)
	require.Equal(t, "started", res.Status)
	require.Len(t, eng.seen, 1)
	require.Contains(t, eng.seen[0], "10.0.0.5", "the file must already hold the database list when the engine starts")
	require.NotContains(t, eng.seen[0], "stale archive copy")
}

func TestCrowdsecLifecycle_AlreadyRunningRewritesWhitelistAndReloadsOnce(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	eng := &recordingEngine{running: true}
	h, db := whitelistDBHandler(t, eng, dir)
	seedWhitelistEntry(t, db, "10.0.0.6")
	path := filepath.Join(dir, managedWhitelistRel)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("stale\n"), 0o600))
	var signals int
	h.signalProcess = func(int, syscall.Signal) error {
		signals++
		return nil
	}

	res, err := h.StartCrowdSec(context.Background())

	require.NoError(t, err)
	require.Equal(t, "already_running", res.Status)
	b, readErr := os.ReadFile(path) //nolint:gosec // G304: test fixture path
	require.NoError(t, readErr)
	require.Contains(t, string(b), "10.0.0.6")
	require.Equal(t, 1, signals, "one reload after the rewrite")
}

func TestCrowdsecLifecycle_StartWaitsForDataLockBeforeWritingWhitelist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	eng := &recordingEngine{}
	h, db := whitelistDBHandler(t, eng, dir)
	seedWhitelistEntry(t, db, "10.0.0.7")
	path := filepath.Join(dir, managedWhitelistRel)

	require.NoError(t, h.dataMu.LockContext(context.Background()))
	done := make(chan error, 1)
	go func() {
		_, err := h.StartCrowdSec(context.Background())
		done <- err
	}()

	select {
	case err := <-done:
		t.Fatalf("Start finished while another operation held the data lock: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	require.NoFileExists(t, path, "the whitelist must not be written while the data lock is held elsewhere")

	h.dataMu.Unlock()
	require.NoError(t, <-done)
	b, err := os.ReadFile(path) //nolint:gosec // G304: test fixture path
	require.NoError(t, err)
	require.Contains(t, string(b), "10.0.0.7")
}

func TestCrowdsecLifecycle_WhitelistWriteFailureDoesNotFailStart(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	eng := &recordingEngine{}
	h, db := whitelistDBHandler(t, eng, dir)
	seedWhitelistEntry(t, db, "10.0.0.8")
	// A directory at the file's path makes the atomic write fail.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, managedWhitelistRel), 0o750))

	res, err := h.StartCrowdSec(context.Background())

	require.NoError(t, err)
	require.Equal(t, "started", res.Status)
	require.True(t, eng.isRunning())
}

// importWhitelistArchive is an archive whose Charon whitelist and one other parser carry lists of their own.
func importWhitelistArchive() map[string]string {
	return map[string]string{
		"config/config.yaml":                             importableConfig,
		managedWhitelistRel:                              "name: charon-whitelist\nwhitelist:\n  ip:\n    - \"192.0.2.1\"\n",
		"config/parsers/s02-enrich/operator.yaml":        "name: operator\nwhitelist:\n  reason: operator\n",
		"config/parsers/s02-enrich/broken-parser.yaml":   "whitelist: [\n",
		"config/parsers/s02-enrich/operator-notes.txt":   "whitelist:\n  reason: not a parser\n",
		"config/parsers/s02-enrich/operator-filter.yaml": "name: filter\nfilter: x\n",
	}
}

func TestImportConfig_RegeneratesWhitelistFromDBAndListsOtherParsers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	h, db := whitelistDBHandler(t, &fakeExec{}, dir)
	seedWhitelistEntry(t, db, "10.0.0.9")
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	otherBody := "name: operator\nwhitelist:\n  reason: operator\n"

	w := importStatus(t, r, importWhitelistArchive())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp struct {
		WhitelistRegenerated bool     `json:"whitelist_regenerated"`
		OtherWhitelistFiles  []string `json:"other_whitelist_files"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.WhitelistRegenerated)
	require.Equal(t, []string{"config/parsers/s02-enrich/operator.yaml"}, resp.OtherWhitelistFiles)

	managed := readTreeFile(t, filepath.Join(dir, managedWhitelistRel))
	require.Contains(t, managed, "10.0.0.9", "the file follows this instance's list, not the archive's")
	require.False(t, strings.Contains(managed, "192.0.2.1"))
	require.Equal(t, otherBody, readTreeFile(t, filepath.Join(dir, "config", "parsers", "s02-enrich", "operator.yaml")))
}

func TestImportConfig_EmptyDBListRemovesArchiveWhitelist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	seedRealLayout(t, dir)
	h, _ := whitelistDBHandler(t, &fakeExec{}, dir)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	w := importStatus(t, r, importWhitelistArchive())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoFileExists(t, filepath.Join(dir, managedWhitelistRel))
	require.Contains(t, w.Body.String(), "config/parsers/s02-enrich/operator.yaml")
	require.Contains(t, w.Body.String(), `"whitelist_regenerated":true`)
}
