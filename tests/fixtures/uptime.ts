/**
 * Reusable fixture builders for the "Uptime Monitoring at Scale" E2E specs.
 *
 * The shapes here mirror the batch summary endpoint contract in
 * `docs/plans/current_spec.md`:
 *   - §3.5.2 — `MonitorSummary` / `BeatDTO` (the JSON the server emits, snake_case)
 *   - §3.5.7 — the matching `frontend/src/api/uptime.ts` type additions
 *
 * These are plain data builders with no Playwright imports, so they can be
 * called directly from `page.route` fulfil handlers.
 */

/**
 * One heartbeat as returned inside `MonitorSummary.recent_beats`.
 * Chronological ASC ordering is guaranteed by `makeBeatSeries`. (§3.5.2)
 */
export interface BeatDTO {
  status: 'up' | 'down';
  latency: number;
  created_at: string;
}

/**
 * A single row of `GET /api/v1/uptime/monitors/summary`. (§3.5.2 / §3.5.7)
 *
 * `proxy_host_id` / `remote_server_id` are always present but nullable
 * (used only for UI grouping). `uptime_24h` is always present, null when
 * there is no heartbeat data in the window.
 */
export interface MonitorSummary {
  id: string;
  name: string;
  type: 'http' | 'tcp' | 'orthrus';
  url: string;
  enabled: boolean;
  status: 'up' | 'down' | 'pending' | 'paused';
  latency: number;
  last_check: string | null;
  interval: number;
  proxy_host_id: number | null;
  remote_server_id: number | null;
  uptime_24h: number | null;
  recent_beats: BeatDTO[];
}

/** The `GET /api/v1/uptime/health` shape. (§3.5.5) */
export interface UptimeHealth {
  heartbeats_dropped: number;
  checks_enqueue_dropped: number;
  queue_depth: number;
  worker_pool_size: number;
}

/**
 * Fixed reference instant for the newest beat, so fixtures are deterministic
 * across runs. Callers can override via `endAt`.
 */
export const FIXED_NOW = new Date('2026-08-27T12:00:00.000Z');

export interface BeatSeriesOptions {
  /** Wall-clock of the newest (last) beat. Default: `FIXED_NOW`. */
  endAt?: Date;
  /** Spacing between consecutive beats, seconds. Default 30 (the interval floor). */
  intervalSeconds?: number;
  /** Status of the final (newest) beat. Default `'up'`. */
  trailingStatus?: 'up' | 'down';
  /**
   * Counting back from the newest beat, every Nth beat is `'down'`.
   * 0 (default) means every non-trailing beat is `'up'`.
   */
  downEvery?: number;
  /** Latency reported for `'up'` beats, ms. `'down'` beats always report 0. Default 42. */
  upLatency?: number;
}

/**
 * Builds `n` heartbeats ordered **chronological ASC** (oldest first, newest
 * last) — matching the `recent_beats` ordering the summary endpoint promises.
 */
export function makeBeatSeries(n: number, options: BeatSeriesOptions = {}): BeatDTO[] {
  const {
    endAt = FIXED_NOW,
    intervalSeconds = 30,
    trailingStatus = 'up',
    downEvery = 0,
    upLatency = 42,
  } = options;

  const beats: BeatDTO[] = [];
  // `age` counts back from the newest beat: age 0 is newest, age n-1 is oldest.
  for (let age = n - 1; age >= 0; age -= 1) {
    const isNewest = age === 0;
    const isDown = isNewest
      ? trailingStatus === 'down'
      : downEvery > 0 && age % downEvery === 0;
    const status: 'up' | 'down' = isDown ? 'down' : 'up';
    beats.push({
      status,
      latency: status === 'down' ? 0 : upLatency,
      created_at: new Date(endAt.getTime() - age * intervalSeconds * 1000).toISOString(),
    });
  }
  return beats;
}

export interface SummaryFixtureOptions {
  /** Length of each monitor's `recent_beats`. Default 30 (summary endpoint default). */
  beats?: number;
  /** Newest-beat instant, forwarded to `makeBeatSeries`. Default `FIXED_NOW`. */
  endAt?: Date;
  /** 0-based indices that should render DOWN (status `'down'` + trailing `down` beat). */
  downIndices?: number[];
}

/**
 * Builds an `n`-element `MonitorSummary[]` matching the §3.5.2 schema.
 *
 * Names are zero-padded so the page's alphabetical sort is stable and
 * predictable. Every third monitor is attributed to a proxy host, the next to
 * a remote server, the rest standalone — so the page's grouping logic is
 * exercised. All monitors are enabled and `up` unless listed in `downIndices`.
 */
export function makeSummaryFixture(n: number, options: SummaryFixtureOptions = {}): MonitorSummary[] {
  const { beats = 30, endAt = FIXED_NOW, downIndices = [] } = options;
  const down = new Set(downIndices);

  return Array.from({ length: n }, (_, i): MonitorSummary => {
    const isDown = down.has(i);
    const seq = String(i + 1).padStart(4, '0');
    const bucket = i % 3; // 0 => proxy host, 1 => remote server, 2 => standalone
    const isHttp = i % 2 === 0;

    return {
      id: `mon-${seq}`,
      name: `Scale Monitor ${seq}`,
      type: isHttp ? 'http' : 'tcp',
      url: isHttp
        ? `https://svc-${seq}.example.test/health`
        : `svc-${seq}.example.test:5432`,
      enabled: true,
      status: isDown ? 'down' : 'up',
      latency: isDown ? 0 : 40 + (i % 25),
      last_check: endAt.toISOString(),
      interval: 30,
      proxy_host_id: bucket === 0 ? 1000 + i : null,
      remote_server_id: bucket === 1 ? 2000 + i : null,
      uptime_24h: isDown ? 87.5 : 99.9,
      recent_beats: makeBeatSeries(beats, {
        endAt,
        trailingStatus: isDown ? 'down' : 'up',
        downEvery: isDown ? 7 : 0,
      }),
    };
  });
}

/** Convenience builder for the `GET /api/v1/uptime/health` mock. (§3.5.5) */
export function makeHealthFixture(overrides: Partial<UptimeHealth> = {}): UptimeHealth {
  return {
    heartbeats_dropped: 0,
    checks_enqueue_dropped: 0,
    queue_depth: 0,
    worker_pool_size: 30,
    ...overrides,
  };
}
