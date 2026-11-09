import { useCallback, useSyncExternalStore } from 'react'

// Charon is a client-only SPA, so the server snapshot only matters if the
// hook is ever rendered outside the browser; it reports "no match" there.
const getServerSnapshot = (): boolean => false

/** Tracks whether a CSS media query currently matches, updating on change events. */
export function useMediaQuery(query: string): boolean {
  const subscribe = useCallback(
    (onChange: () => void) => {
      const mq = window.matchMedia(query)
      mq.addEventListener('change', onChange)
      return () => mq.removeEventListener('change', onChange)
    },
    [query]
  )

  const getSnapshot = useCallback(() => window.matchMedia(query).matches, [query])

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot)
}
