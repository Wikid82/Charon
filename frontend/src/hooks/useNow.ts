import { useEffect, useState } from 'react';

/**
 * Returns the current epoch time in milliseconds, refreshed every `intervalMs`
 * while `enabled` is true. Components use this instead of calling `Date.now()`
 * during render, which React Compiler flags as impure and which would not
 * re-render on its own as time passes.
 * @param intervalMs - How often to refresh the clock (default 1000)
 * @param enabled - When false, no timer runs and the last sampled value is returned
 * @returns The most recently sampled `Date.now()` value
 */
export function useNow(intervalMs = 1000, enabled = true): number {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!enabled) return;
    const timer = setInterval(() => setNow(Date.now()), intervalMs);
    return () => clearInterval(timer);
  }, [enabled, intervalMs]);

  return now;
}
