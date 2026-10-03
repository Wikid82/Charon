# Database Performance

**Audience:** contributors. This page is not published on the docs site.
**Scope:** the uptime summary refresh (`UptimeSummaryService.GetSummary`, GH #1441, database slice of #42) and the facts behind it.

## What was slow

`GET /api/v1/uptime/monitors/summary` is served from a 30 s in-memory cache. On a cache miss it runs three queries. Two of them (the sparkline beats and the 24 h uptime) used a `ROW_NUMBER()` window and a table-wide `GROUP BY`, which the SQLite planner answered by scanning every retained heartbeat instead of only the last 24 hours. Charon uses a single database connection, so the whole scan blocked heartbeat ingestion and every API request behind it.

The queries are now per monitor: each monitor's newest N beats and its two counts are index range reads on `idx_heartbeat_monitor_created (monitor_id, created_at)`, so cost follows monitors x window instead of the retained history.

## Method

- Workload: 150 proxy hosts / 150 uptime monitors (60 s interval) / **1,512,000 heartbeats** (7 days x 1,440 x 150, 98 % `up`; 216,000 rows in the trailing 24 h).
- Hardware class: "dev host" (a single development machine, not a Raspberry Pi; Pi numbers are unmeasured).
- Database: opened through `database.Connect` (pure-Go SQLite driver, production pragmas: WAL, `synchronous=NORMAL`, 64 MB cache, one open connection), seeded through the real models and `AutoMigrate`, on a scratch file outside `/tmp`.
- Timing: cold `GetSummary` (cache miss), min / median of 7 runs, old SQL vs new SQL on the same database, first without and then with `idx_heartbeat_monitor_created`.
- Reproduce a smaller version with `go test ./internal/services -run XXX -bench UptimeSummaryCold -benchtime=5x` (50 monitors x 2 days; scale the `monitorCount` and `days` constants in `BenchmarkUptimeSummaryCold` (`uptime_summary_equivalence_test.go`) for the full workload).
- Absolute numbers vary 20-40 % between sessions (page cache, WAL state). Read the ratios, not single figures.

## Results

| Cold `GetSummary` | Old SQL (min / median) | New SQL (min / median) |
| --- | --- | --- |
| With `idx_heartbeat_monitor_created` (steady state of every install after the pruner's first clean pass) | 1.61 s / 1.72 s | **109 ms / 130 ms** |
| Without the index | 1.77 s / 2.06 s | 1.49 s / 1.65 s |

Per query (min / median):

| Query | Old, with index | New, with index | Old, no index | New, no index |
| --- | --- | --- | --- | --- |
| Recent beats | 1.06 s / 1.15 s | 39 ms / 44 ms | 1.31 s / 1.40 s | 0.80 s / 0.87 s |
| 24 h uptime | 514 ms / 540 ms | 47 ms / 57 ms | 579 ms / 589 ms | 600 ms / 633 ms |

Measured by the author of the change, same session, same database. The design phase measured the same shape earlier (recent beats 30-44 ms, uptime 45-53 ms with the index; total 1.4 s down to roughly 0.08-0.10 s) and agrees within the session-to-session spread above.

**Without the index** the refresh total is better (about 20 % in the design measurements: 1.21-1.37 s vs 1.54-1.71 s; about 15 % here) but the uptime query alone is **slower** (about 11-18 % in the design measurements, 4-8 % in the table above; two correlated counts each scan `idx_heartbeat_lookup (monitor_id=?)`, about the same rows as the old scan plus per-monitor overhead). That state is transient: the pruner builds the index after its first clean caught-up pass, and a fresh install has few rows. The index is deliberately not in the model tags, so an upgrade does not index millions of rows inside `AutoMigrate`.

A query-plan guard (`TestUptimeSummary_QueryPlanUsesCompositeIndex`) fails if the plan with the index ever scans `uptime_heartbeats` again or sorts more than the final output.

### Ties on `created_at`

The old window query was nondeterministic when several heartbeats share a timestamp, and returned different rows with and without the composite index. The new queries add `id` as tie-breaker (`created_at DESC, id DESC` inside the cap, `created_at ASC, id ASC` in the output), at no measurable cost. Equivalence tests prove byte-identical JSON against the untouched legacy SQL on tie-free data and against a tie-broken legacy oracle on tie-heavy data, in both index states.

## Alternatives measured and rejected

- **Single-pass 24 h uptime** (design measurements, with / without the composite index): one scalar subquery per monitor with a conditional `SUM`: 125-150 ms / 517-553 ms; `JOIN` + `GROUP BY`: 240-253 ms / 627-681 ms; the old shape restricted to the window and forced onto `idx_uptime_heartbeats_created_at`: 258-305 ms / 258-281 ms. A single pass needs `status`, which the composite index does not hold, so each pays a row lookup per in-window heartbeat (216,000). Two covering range seeks (the chosen form: 45-53 ms) are 3-6x faster in the steady state; the scalar-subquery form is only 5-10 % better without the index.
- **Covering index `(created_at, monitor_id, status)`**: no gain (the planner still picks `idx_heartbeat_lookup`), +76 MB and about +30 % insert cost (341 vs 259 ms per 20,000 rows). Rejected.
- **`ANALYZE` alone**: the old uptime query improves to about 60-70 ms but the window-function query stays at about 0.5 s; the rewrite does not depend on statistics.

## Index sizes at 1,512,000 heartbeats

| Object | Real install (AutoMigrate before load) | Freshly built (lower bound) |
| --- | --- | --- |
| `idx_heartbeat_lookup (monitor_id, status, created_at)` | 233.9 MB | 123.2 MB |
| `idx_heartbeat_monitor_created (monitor_id, created_at)` | 118.2 MB | 118.2 MB |
| `idx_uptime_heartbeats_created_at (created_at)` | 75.3 MB | 64.2 MB |
| table `uptime_heartbeats` | 128.7 MB | 128.7 MB |

Source: `dbstat` on a database with the production schema. The middle column is the primary figure: GORM indexes created by `AutoMigrate` before the data arrives (the real upgrade and fresh-install path) fragment those two b-trees, while the composite is built after the load. The last column is a database whose indexes were all created after the load; it is the lower bound, not what installs normally show.

## Known gap: no `singleflight` on a cache miss

`GetSummary` has no request coalescing: concurrent requests arriving right after the 30 s cache expires each run the three queries. With the refresh at roughly 100 ms this is tolerable and was not the measured hot spot, so it is recorded here as a possible follow-up rather than changed.

## Other measured paths (no action needed)

`ProxyHostService.List` with 150 hosts: 3-6 ms, six queries (no N+1). `GetMonitorHistory(60)` for one monitor: under 0.3 ms. Per-request `Cerberus.IsEnabled` plus the user lookup: about 0.1 ms each when idle, all index searches; they only suffer when queued behind a long statement on the single connection, which the rewrite removes.
