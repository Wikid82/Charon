import { useSyncExternalStore } from 'react'

/**
 * Tiny module-level store for "the server is optimizing its database".
 *
 * It lives outside React because the Axios response interceptor (which has no
 * component context) must be able to flip it, while `MaintenanceGate` renders
 * from it.
 */
let active = false
const listeners = new Set<() => void>()

const notify = () => {
  for (const listener of listeners) listener()
}

export const enterMaintenance = (): void => {
  if (active) return
  active = true
  notify()
}

export const leaveMaintenance = (): void => {
  if (!active) return
  active = false
  notify()
}

export const isMaintenanceActive = (): boolean => active

const subscribe = (listener: () => void) => {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** Re-renders the caller whenever maintenance mode starts or ends. */
export const useMaintenanceMode = (): boolean =>
  useSyncExternalStore(subscribe, isMaintenanceActive, isMaintenanceActive)

/**
 * True for the gate's `503 {"maintenance": true}` answer. Unlike a 401 this is
 * never a session problem: callers must not log the user out or retry.
 */
export const isMaintenanceError = (error: unknown): boolean => {
  if (typeof error !== 'object' || error === null) return false
  const response = (error as { response?: { status?: number; data?: unknown } }).response
  if (response?.status !== 503) return false
  const data = response.data
  return typeof data === 'object' && data !== null && (data as { maintenance?: unknown }).maintenance === true
}
