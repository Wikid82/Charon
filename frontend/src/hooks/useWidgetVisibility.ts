import { useState, useCallback } from 'react'

export type WidgetKey =
  | 'requestCount'
  | 'trafficVolume'
  | 'topHosts'
  | 'statusDistribution'
  | 'certExpiry'
  | 'serviceHealth'

const STORAGE_KEY = 'charon.stats.widgetVisibility'

const ALL_WIDGETS: WidgetKey[] = [
  'requestCount',
  'trafficVolume',
  'topHosts',
  'statusDistribution',
  'certExpiry',
  'serviceHealth',
]

const DEFAULT_VISIBILITY: Record<WidgetKey, boolean> = {
  requestCount: true,
  trafficVolume: true,
  topHosts: true,
  statusDistribution: true,
  certExpiry: true,
  serviceHealth: true,
}

function loadFromStorage(): Record<WidgetKey, boolean> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return { ...DEFAULT_VISIBILITY }
    const parsed: unknown = JSON.parse(raw)
    if (typeof parsed !== 'object' || parsed === null) return { ...DEFAULT_VISIBILITY }
    const result = { ...DEFAULT_VISIBILITY }
    for (const key of ALL_WIDGETS) {
      const val = (parsed as Record<string, unknown>)[key]
      if (typeof val === 'boolean') {
        result[key] = val
      }
    }
    return result
  } catch {
    return { ...DEFAULT_VISIBILITY }
  }
}

function saveToStorage(visibility: Record<WidgetKey, boolean>): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(visibility))
  } catch {
    // Ignore storage errors (e.g. private browsing quota)
  }
}

export function useWidgetVisibility() {
  const [visibility, setVisibility] = useState<Record<WidgetKey, boolean>>(loadFromStorage)

  const toggleWidget = useCallback((key: WidgetKey) => {
    setVisibility((prev) => {
      const next = { ...prev, [key]: !prev[key] }
      saveToStorage(next)
      return next
    })
  }, [])

  const resetAll = useCallback(() => {
    const defaults = { ...DEFAULT_VISIBILITY }
    saveToStorage(defaults)
    setVisibility(defaults)
  }, [])

  return { visibility, toggleWidget, resetAll }
}
