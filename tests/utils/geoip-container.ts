/**
 * Hides and restores the GeoIP country database inside the E2E container, so a spec can run the "database missing"
 * path without rebuilding the image.
 *
 * Both copies the application can load are moved, not deleted: the bundled copy under /app/geoip and the data-folder
 * copy under /app/data/geoip. A move keeps the file's contents, so restoring is exact. Only files this process moved
 * are moved back; a copy the spec never hid is left alone. A leftover from an interrupted run (a `.e2e-hidden` file
 * with no original beside it) is moved back the next time a spec hides or restores.
 *
 * Commands run as root (`docker exec -u 0`) because the bundled copy is owned by the application user.
 */

import { execInContainer, findE2EContainer } from './container-exec';

export const BUNDLED_DATABASE_PATH = '/app/geoip/GeoLite2-Country.mmdb';
export const DATA_DATABASE_PATH = '/app/data/geoip/GeoLite2-Country.mmdb';

const DATABASE_PATHS: readonly string[] = [BUNDLED_DATABASE_PATH, DATA_DATABASE_PATH];
const HIDDEN_SUFFIX = '.e2e-hidden';

/** Database paths this process moved away. Restoring moves back only these. */
const hiddenByThisProcess = new Set<string>();

function execAsRoot(container: string, args: readonly string[]): string {
  return execInContainer(container, args, { user: '0' });
}

function fileExists(container: string, path: string): boolean {
  // The path is passed as a positional argument, never interpolated into the script.
  const answer = execAsRoot(container, [
    'sh',
    '-c',
    'if [ -e "$1" ]; then echo yes; else echo no; fi',
    'sh',
    path,
  ]);
  return answer.trim() === 'yes';
}

function movePath(container: string, from: string, to: string): void {
  execAsRoot(container, ['mv', from, to]);
}

/** Moves back the copies an interrupted run left hidden. A copy already in place is never overwritten. */
function restoreLeftovers(container: string): void {
  for (const path of DATABASE_PATHS) {
    const hidden = `${path}${HIDDEN_SUFFIX}`;
    if (fileExists(container, hidden) && !fileExists(container, path)) {
      movePath(container, hidden, path);
    }
  }
}

/** Moves back any database a previous interrupted run left hidden. Call before a spec needs the database present. */
export function restoreLeftoverGeoIPDatabases(): void {
  restoreLeftovers(findE2EContainer());
}

/**
 * Hides every GeoIP country database copy the container has. Leftovers from an interrupted run are restored first.
 * Returns true when at least one copy was hidden.
 */
export function hideGeoIPDatabases(): boolean {
  const container = findE2EContainer();
  restoreLeftovers(container);

  let hiddenAny = false;
  for (const path of DATABASE_PATHS) {
    if (!fileExists(container, path)) {
      continue;
    }
    movePath(container, path, `${path}${HIDDEN_SUFFIX}`);
    hiddenByThisProcess.add(path);
    hiddenAny = true;
  }
  return hiddenAny;
}

/**
 * Moves back the copies this process hid. Returns true when at least one copy was moved back. Safe to call when
 * nothing was hidden.
 */
export function restoreGeoIPDatabases(): boolean {
  const container = findE2EContainer();

  let restoredAny = false;
  for (const path of hiddenByThisProcess) {
    const hidden = `${path}${HIDDEN_SUFFIX}`;
    if (fileExists(container, hidden)) {
      movePath(container, hidden, path);
      restoredAny = true;
    }
  }
  hiddenByThisProcess.clear();
  return restoredAny;
}
