package services

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

const (
	// uptimeSchedulerTick is how often the scheduler scans its in-memory
	// schedules for due work.
	uptimeSchedulerTick = 5 * time.Second

	// uptimeSchedulerBackfillWindow is the spread applied to past-due / zero
	// NextCheckAt monitors at cold start so a restart does not stampede every
	// monitor onto the first tick (spec §3.1.2).
	uptimeSchedulerBackfillWindow = 60 * time.Second

	// uptimeSchedulerMaxEnqueuePerTick caps host + monitor enqueues per tick so
	// one tick can never flood the bounded pool queue.
	uptimeSchedulerMaxEnqueuePerTick = 200

	// uptimeSchedulerRescanEveryTicks runs the new/disabled-monitor reconcile
	// every Nth tick (~30s at a 5s tick).
	uptimeSchedulerRescanEveryTicks = 6

	// uptimeSchedulerFlagTTL is how long the feature-flag lookup is cached.
	uptimeSchedulerFlagTTL = 60 * time.Second
)

// UptimeScheduler drives per-monitor checks from each monitor's own
// NextCheckAt, replacing the global 60s ticker (spec §3.1). It keeps two
// in-memory schedules — one keyed by monitor id, one by host id — advances them
// by the per-monitor / per-host interval as it enqueues onto the bounded worker
// pool, and batches the monitor NextCheckAt column write-back.
type UptimeScheduler struct {
	db        *gorm.DB
	pool      *UptimeWorkerPool
	uptimeCfg *uptimeConfig
	tick      time.Duration

	mu           sync.Mutex
	monSchedule  map[string]time.Time // monitorID -> next due (in-memory source of truth)
	hostSchedule map[string]time.Time // hostID    -> next due (in-memory only; not persisted)
	hostMinInt   map[string]int       // hostID    -> min(enabled child intervals), clamped
	writeback    map[string]time.Time // pending uptime_monitors.next_check_at persists
	known        map[string]struct{}  // hydrated monitor ids (for the new-monitor rescan)
	tickCount    int

	// feature-flag cache (spec §3.1.2 — cached 60s)
	flagMu      sync.Mutex
	flagVal     bool
	flagExpires time.Time

	now func() time.Time // injectable clock — test seam
}

// NewUptimeScheduler builds a scheduler over pool's db + config. Run is started
// by routes.go (C5).
func NewUptimeScheduler(pool *UptimeWorkerPool) *UptimeScheduler {
	return &UptimeScheduler{
		db:           pool.db,
		pool:         pool,
		uptimeCfg:    pool.cfg,
		tick:         uptimeSchedulerTick,
		monSchedule:  make(map[string]time.Time),
		hostSchedule: make(map[string]time.Time),
		hostMinInt:   make(map[string]int),
		writeback:    make(map[string]time.Time),
		known:        make(map[string]struct{}),
		now:          time.Now,
	}
}

// Run hydrates the schedules, seeds the pool's debounce maps, then loops every
// tick enqueuing due work until ctx is cancelled. On cancellation it does one
// best-effort write-back flush and returns — the first link of the S4 teardown
// chain (spec §3.1.4): no further enqueues happen after this.
func (s *UptimeScheduler) Run(ctx context.Context) {
	s.hydrate(ctx)
	if s.pool != nil {
		if err := s.pool.SeedState(ctx); err != nil {
			logger.Log().WithError(err).Error("UptimeScheduler: pool SeedState failed; debounce maps start empty")
		}
	}

	ticker := time.NewTicker(s.tick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.flushWriteback(context.Background()) // best-effort final persist
			return
		case <-ticker.C:
			if !s.featureEnabled(ctx) {
				continue
			}
			s.runTick(ctx)
		}
	}
}

// Rehydrate re-runs cold-start hydration and re-seeds the pool's debounce maps.
// Called from the backup-restore reconcile path so a live restore re-syncs the
// in-memory schedule immediately instead of waiting for the ~30s rescan (S6).
func (s *UptimeScheduler) Rehydrate(ctx context.Context) {
	s.hydrate(ctx)
	if s.pool != nil {
		if err := s.pool.ReseedState(ctx); err != nil {
			logger.Log().WithError(err).Error("UptimeScheduler: ReseedState after restore failed")
		}
	}
	logger.Log().Info("UptimeScheduler: rehydrated after backup restore")
}

// --- hydration ---

type schedMonRow struct {
	ID           string
	Interval     int
	NextCheckAt  time.Time
	UptimeHostID *string
}

// hydrate rebuilds monSchedule / hostSchedule / hostMinInt / known from the DB.
// Maps are built outside s.mu and swapped in under it (supervisor change #4).
func (s *UptimeScheduler) hydrate(ctx context.Context) {
	now := s.now()

	var mrows []schedMonRow
	if err := s.db.WithContext(ctx).Model(&models.UptimeMonitor{}).
		Select("id", "interval", "next_check_at", "uptime_host_id").
		Where("enabled = ?", true).Scan(&mrows).Error; err != nil {
		logger.Log().WithError(err).Error("UptimeScheduler: monitor hydration failed")
		return
	}

	monSchedule := make(map[string]time.Time, len(mrows))
	known := make(map[string]struct{}, len(mrows))
	backfilled := make([]string, 0)
	for _, r := range mrows {
		eff := clampInterval(r.Interval, s.uptimeCfg)
		due := r.NextCheckAt
		if !due.After(now) { // past or zero → jittered backfill over the window
			due = now.Add(jitterDuration(minDuration(time.Duration(eff)*time.Second, uptimeSchedulerBackfillWindow)))
			backfilled = append(backfilled, r.ID)
		}
		monSchedule[r.ID] = due
		known[r.ID] = struct{}{}
	}

	hostMinInt, hostSchedule := s.loadHostSchedules(ctx, now)

	s.mu.Lock()
	s.monSchedule = monSchedule
	s.hostSchedule = hostSchedule
	s.hostMinInt = hostMinInt
	s.known = known
	for _, id := range backfilled {
		s.writeback[id] = monSchedule[id]
	}
	s.mu.Unlock()

	s.flushWriteback(ctx)
}

type schedHostRow struct {
	ID          string
	MinInterval int
}

// loadHostSchedules computes each host's effective interval (min of its enabled
// child monitor intervals, clamped) and a jittered first due-time. Host
// due-times are in-memory only — no uptime_hosts column, no write-back.
func (s *UptimeScheduler) loadHostSchedules(ctx context.Context, now time.Time) (map[string]int, map[string]time.Time) {
	var hrows []schedHostRow
	if err := s.db.WithContext(ctx).Model(&models.UptimeMonitor{}).
		Select("uptime_host_id AS id, MIN(interval) AS min_interval").
		Where("enabled = ? AND uptime_host_id IS NOT NULL AND uptime_host_id != ''", true).
		Group("uptime_host_id").Scan(&hrows).Error; err != nil {
		logger.Log().WithError(err).Error("UptimeScheduler: host hydration failed")
		return make(map[string]int), make(map[string]time.Time)
	}

	hostMinInt := make(map[string]int, len(hrows))
	hostSchedule := make(map[string]time.Time, len(hrows))
	for _, r := range hrows {
		eff := clampInterval(r.MinInterval, s.uptimeCfg)
		hostMinInt[r.ID] = eff
		hostSchedule[r.ID] = now.Add(jitterDuration(minDuration(time.Duration(eff)*time.Second, uptimeSchedulerBackfillWindow)))
	}
	return hostMinInt, hostSchedule
}

// --- per-tick ---

func (s *UptimeScheduler) runTick(ctx context.Context) {
	now := s.now()
	s.tickCount++

	s.hostPass(ctx, now)
	s.monitorPass(ctx, now)

	if s.tickCount%uptimeSchedulerRescanEveryTicks == 0 {
		s.rescan(ctx)
	}

	s.flushWriteback(ctx)
}

// hostPass enqueues due host connectivity checks and advances hostSchedule.
func (s *UptimeScheduler) hostPass(ctx context.Context, now time.Time) {
	s.mu.Lock()
	due := make([]string, 0)
	for hid, t := range s.hostSchedule {
		if !t.After(now) {
			due = append(due, hid)
		}
	}
	s.mu.Unlock()
	if len(due) == 0 {
		return
	}
	if len(due) > uptimeSchedulerMaxEnqueuePerTick {
		due = due[:uptimeSchedulerMaxEnqueuePerTick]
	}

	for _, h := range s.loadHostSnapshots(ctx, due) {
		hh := h
		if s.pool != nil && s.pool.TryEnqueue(UptimeJob{Kind: JobHostCheck, Host: &hh}) {
			s.mu.Lock()
			iv := clampInterval(s.hostMinInt[hh.ID], s.uptimeCfg)
			s.hostSchedule[hh.ID] = now.Add(time.Duration(iv) * time.Second)
			s.mu.Unlock()
		}
		// else: leave due, retried next tick
	}
}

// monitorPass enqueues due monitor checks (skipping tcp monitors of known-down
// hosts), advances monSchedule and stages the NextCheckAt write-back.
func (s *UptimeScheduler) monitorPass(ctx context.Context, now time.Time) {
	type dueMon struct {
		id  string
		due time.Time
	}
	s.mu.Lock()
	pending := make([]dueMon, 0)
	for id, t := range s.monSchedule {
		if !t.After(now) {
			pending = append(pending, dueMon{id, t})
		}
	}
	s.mu.Unlock()
	if len(pending) == 0 {
		return
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].due.Before(pending[j].due) })
	if len(pending) > uptimeSchedulerMaxEnqueuePerTick {
		pending = pending[:uptimeSchedulerMaxEnqueuePerTick]
	}

	ids := make([]string, len(pending))
	for i, p := range pending {
		ids[i] = p.id
	}

	for _, m := range s.loadJobSnapshots(ctx, ids) {
		next := now.Add(time.Duration(clampInterval(m.Interval, s.uptimeCfg)) * time.Second)

		if s.hostDownSkips(m) {
			s.advanceMonitor(m.ID, next, true)
			continue
		}
		if s.pool != nil && s.pool.TryEnqueue(UptimeJob{Kind: JobMonitorCheck, Monitor: m}) {
			s.advanceMonitor(m.ID, next, true)
		}
		// else: leave due, retried next tick
	}
}

// hostDownSkips reports whether m is a tcp monitor whose parent host is
// currently known-down — the host check drives its recovery, so no monitor
// check (or heartbeat) is enqueued while the host is down (spec §3.2.3).
func (s *UptimeScheduler) hostDownSkips(m models.UptimeMonitor) bool {
	if s.pool == nil {
		return false
	}
	if strings.ToLower(strings.TrimSpace(m.Type)) != "tcp" || m.UptimeHostID == nil || *m.UptimeHostID == "" {
		return false
	}
	st, ok := s.pool.HostState(*m.UptimeHostID)
	return ok && st.status == "down"
}

func (s *UptimeScheduler) advanceMonitor(id string, next time.Time, persist bool) {
	s.mu.Lock()
	s.monSchedule[id] = next
	if persist {
		s.writeback[id] = next
	}
	s.mu.Unlock()
}

// --- rescan ---

type rescanMonRow struct {
	ID          string
	Interval    int
	NextCheckAt time.Time
}

// rescan picks up monitors created/enabled since boot, drops disabled ones, and
// recomputes host schedules (spec §3.1.2 step (c)).
func (s *UptimeScheduler) rescan(ctx context.Context) {
	now := s.now()

	var rows []rescanMonRow
	if err := s.db.WithContext(ctx).Model(&models.UptimeMonitor{}).
		Select("id", "interval", "next_check_at").
		Where("enabled = ?", true).Scan(&rows).Error; err != nil {
		logger.Log().WithError(err).Error("UptimeScheduler: rescan query failed")
		return
	}

	enabled := make(map[string]struct{}, len(rows))
	added := make([]string, 0)

	s.mu.Lock()
	for _, r := range rows {
		enabled[r.ID] = struct{}{}
		if _, ok := s.known[r.ID]; ok {
			continue
		}
		eff := clampInterval(r.Interval, s.uptimeCfg)
		due := r.NextCheckAt
		if !due.After(now) {
			due = now.Add(jitterDuration(minDuration(time.Duration(eff)*time.Second, uptimeSchedulerBackfillWindow)))
		}
		s.monSchedule[r.ID] = due
		s.known[r.ID] = struct{}{}
		s.writeback[r.ID] = due
		added = append(added, r.ID)
	}
	for id := range s.known {
		if _, ok := enabled[id]; !ok {
			delete(s.known, id)
			delete(s.monSchedule, id)
		}
	}
	s.mu.Unlock()

	if s.pool != nil {
		for _, id := range added {
			s.pool.EnsureMonitorState(id)
		}
	}

	hostMinInt, hostSchedule := s.loadHostSchedules(ctx, now)
	s.mu.Lock()
	s.hostMinInt = hostMinInt
	for hid, due := range hostSchedule {
		if _, ok := s.hostSchedule[hid]; !ok {
			s.hostSchedule[hid] = due // new host — adopt jittered due-time
		}
	}
	for hid := range s.hostSchedule {
		if _, ok := hostMinInt[hid]; !ok {
			delete(s.hostSchedule, hid) // host no longer has enabled child monitors
		}
	}
	s.mu.Unlock()
}

// --- write-back ---

// flushWriteback persists staged NextCheckAt values, one grouped UPDATE per
// distinct timestamp, all in a single transaction. On failure the entries are
// re-staged for the next tick (in-memory monSchedule stays the runtime truth).
func (s *UptimeScheduler) flushWriteback(ctx context.Context) {
	s.mu.Lock()
	if len(s.writeback) == 0 {
		s.mu.Unlock()
		return
	}
	pending := s.writeback
	s.writeback = make(map[string]time.Time)
	s.mu.Unlock()

	groups := make(map[time.Time][]string)
	for id, t := range pending {
		groups[t] = append(groups[t], id)
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for t, ids := range groups {
			if e := tx.Model(&models.UptimeMonitor{}).
				Where("id IN ?", ids).
				Update("next_check_at", t).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if err != nil {
		logger.Log().WithError(err).Warn("UptimeScheduler: next_check_at write-back failed; will retry next tick")
		s.mu.Lock()
		for id, t := range pending {
			if _, superseded := s.writeback[id]; !superseded {
				s.writeback[id] = t
			}
		}
		s.mu.Unlock()
	}
}

// --- snapshots / helpers ---

func (s *UptimeScheduler) loadJobSnapshots(ctx context.Context, ids []string) []models.UptimeMonitor {
	if len(ids) == 0 {
		return nil
	}
	var monitors []models.UptimeMonitor
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&monitors).Error; err != nil {
		logger.Log().WithError(err).Error("UptimeScheduler: loadJobSnapshots failed")
		return nil
	}
	return monitors
}

func (s *UptimeScheduler) loadHostSnapshots(ctx context.Context, ids []string) []models.UptimeHost {
	if len(ids) == 0 {
		return nil
	}
	var hosts []models.UptimeHost
	if err := s.db.WithContext(ctx).Where("id IN ?", ids).Find(&hosts).Error; err != nil {
		logger.Log().WithError(err).Error("UptimeScheduler: loadHostSnapshots failed")
		return nil
	}
	return hosts
}

func (s *UptimeScheduler) featureEnabled(ctx context.Context) bool {
	s.flagMu.Lock()
	defer s.flagMu.Unlock()
	if !s.flagExpires.IsZero() && s.now().Before(s.flagExpires) {
		return s.flagVal
	}
	val := true
	var setting models.Setting
	if err := s.db.WithContext(ctx).Where("key = ?", "feature.uptime.enabled").First(&setting).Error; err == nil {
		val = setting.Value == "true"
	}
	s.flagVal = val
	s.flagExpires = s.now().Add(uptimeSchedulerFlagTTL)
	return val
}

// jitterDuration returns a uniformly random duration in [0, maxD). A
// non-positive maxD yields 0. Uses crypto/rand (cheap here — a few hundred calls
// per hydrate) so the security scanners stay happy about randomness sources.
func jitterDuration(maxD time.Duration) time.Duration {
	if maxD <= 0 {
		return 0
	}
	var b [8]byte
	if _, err := crand.Read(b[:]); err != nil {
		return maxD / 2 // deterministic fallback; scheduling jitter is non-critical
	}
	return time.Duration(binary.BigEndian.Uint64(b[:]) % uint64(maxD))
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
