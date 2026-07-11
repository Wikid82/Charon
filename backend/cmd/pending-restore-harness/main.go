// Command pending-restore-harness is test infrastructure ONLY (Issue #32
// gap-closing plan, QA finding 2b) — it is not shipped, not referenced by
// any production code path, and not part of the "charon" server binary.
//
// It exists so a Go test can prove, across a REAL OS process boundary
// (not just an in-process function call), that a durable pending-restore
// marker left by services.BackupService.RestoreBackupSafe's F3 fallback
// (spec §3.5) is actually consumed on the next process start — and,
// specifically, that database.ApplyPendingRestore runs BEFORE
// database.Connect ever opens the live database file, mirroring
// backend/cmd/api/main.go's own startup ordering (main.go:214-218) exactly.
//
// Two invocations simulate a real restart:
//
//	pending-restore-harness -mode=prep -db=<dbPath> -source=<validatedDbPath>
//	    Copies source's bytes to <dbPath>.pending-restore and fsyncs, exactly
//	    like BackupService.writePendingRestoreFile does — simulating a
//	    process that validated a replacement database, persisted the durable
//	    F3 marker, and then died/exited before its live rehydrate could
//	    apply it in-process.
//
//	pending-restore-harness -mode=boot -db=<dbPath>
//	    Runs the exact two production calls, in the exact production order:
//	    database.ApplyPendingRestore(dbPath) followed by database.Connect(dbPath).
//	    This is the "fresh process boot" half — if a future edit to main.go
//	    ever reordered these two calls, this mode (and therefore the test
//	    driving it) would stop observing the swap and fail.
//
// Every outcome is reported on a single stdout line starting with "RESULT:"
// so the driving test can assert on it without depending on log formatting.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/logger"
)

func main() {
	mode := flag.String("mode", "", "prep | boot")
	dbPath := flag.String("db", "", "path to the live database file")
	sourcePath := flag.String("source", "", "prep mode only: path to the already-validated replacement database")
	flag.Parse()

	// Discard logger output so it doesn't interleave with our RESULT line on
	// stdout; the test only cares about the RESULT line and the on-disk
	// database file this process leaves behind.
	logger.Init(false, io.Discard)

	switch *mode {
	case "prep":
		if err := prep(*dbPath, *sourcePath); err != nil {
			fmt.Printf("RESULT: prep-failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("RESULT: prep-ok")
	case "boot":
		bootExitCode := boot(*dbPath)
		os.Exit(bootExitCode)
	default:
		fmt.Println("RESULT: usage-error: -mode must be prep or boot")
		os.Exit(2)
	}
}

// prep simulates the tail end of RestoreBackupSafe's F3 fallback
// (writePendingRestoreFile, backend/internal/services/backup_restore_safe.go)
// without importing the services package (which would pull in cron/gorm/etc.
// dependencies this minimal harness has no need for) — it performs the exact
// same file operation: copy the validated replacement database's bytes to
// dbPath+".pending-restore", 0600, fsync'd.
func prep(dbPath, sourcePath string) error {
	if dbPath == "" || sourcePath == "" {
		return fmt.Errorf("-db and -source are both required in prep mode")
	}

	src, err := os.Open(sourcePath) // #nosec G304 -- test harness, paths supplied by the driving test
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer func() { _ = src.Close() }()

	pendingPath := dbPath + ".pending-restore"
	dst, err := os.OpenFile(pendingPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- test harness, paths supplied by the driving test
	if err != nil {
		return fmt.Errorf("create pending-restore file: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("write pending-restore file: %w", err)
	}

	return dst.Sync()
}

// boot replicates backend/cmd/api/main.go's startup ordering verbatim: call
// database.ApplyPendingRestore(dbPath) BEFORE database.Connect(dbPath) ever
// opens the live database (main.go:208-221). Returns a process exit code
// (0 success, non-zero failure) rather than calling os.Exit directly so
// deferred cleanup (closing the gorm connection) always runs first.
func boot(dbPath string) int {
	if dbPath == "" {
		fmt.Println("RESULT: usage-error: -db is required in boot mode")
		return 2
	}

	// This is the exact call main.go makes immediately before
	// database.Connect — see main.go's comment at the call site for why the
	// ordering matters (no live WAL pool must exist when the swap happens).
	pendingErr := database.ApplyPendingRestore(dbPath)

	db, connectErr := database.Connect(dbPath)
	if connectErr != nil {
		fmt.Printf("RESULT: connect-failed: %v (pending-restore error: %v)\n", connectErr, pendingErr)
		return 1
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer func() { _ = sqlDB.Close() }()
	}

	if pendingErr != nil {
		fmt.Printf("RESULT: boot-ok-pending-restore-failed: %v\n", pendingErr)
		return 0
	}

	fmt.Println("RESULT: boot-ok-pending-restore-applied-or-absent")
	return 0
}
