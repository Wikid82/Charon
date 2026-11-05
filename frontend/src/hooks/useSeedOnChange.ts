import { useState } from 'react'

/**
 * Calls `onChange` during render whenever `source` changes identity, including on mount.
 * Use it to seed local form state from server data. It replaces
 * `useEffect(() => { if (source) ... }, [source])`, so the stale first render is never committed.
 * `source` must be a stable reference (query data or state), or the hook re-seeds on every render.
 * Falsy sources are recorded but not passed on, matching the old `if (source)` guard.
 */
export function useSeedOnChange<T>(source: T, onChange: (value: NonNullable<T>) => void): void {
  const [seen, setSeen] = useState<{ source: T } | null>(null)
  if (seen === null || seen.source !== source) {
    setSeen({ source })
    if (source) onChange(source)
  }
}
