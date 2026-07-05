import { useEffect, useState } from 'react';

/**
 * Returns a debounced copy of a value that only updates after the value
 * has stayed unchanged for the given delay. Used to avoid firing a server
 * query on every keystroke of a search input.
 * @param value - The rapidly changing source value
 * @param delayMs - Debounce delay in milliseconds (default 300)
 * @returns The debounced value
 */
export function useDebouncedValue<T>(value: T, delayMs = 300): T {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => setDebounced(value), delayMs);
    return () => clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}
