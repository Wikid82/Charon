const PLACEHOLDER = '—'

function parseTime(dateStr: string): number {
  return dateStr ? new Date(dateStr).getTime() : Number.NaN
}

/** Relative time ("5m ago"); returns '—' when the value is empty or unparseable. */
export function formatRelativeTime(dateStr: string): string {
  const then = parseTime(dateStr)
  if (Number.isNaN(then)) return PLACEHOLDER

  const diffMs = Date.now() - then
  if (diffMs < 0) return 'just now'

  const seconds = Math.floor(diffMs / 1000)
  if (seconds < 60) return `${seconds}s ago`

  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`

  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours}h ago`

  return `${Math.floor(hours / 24)}d ago`
}

/** Localized absolute time for tooltips; undefined when the value is empty or unparseable. */
export function formatAbsoluteTime(dateStr: string): string | undefined {
  const then = parseTime(dateStr)
  return Number.isNaN(then) ? undefined : new Date(then).toLocaleString()
}
