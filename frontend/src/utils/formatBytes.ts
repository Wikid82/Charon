const UNITS = ['B', 'KB', 'MB', 'GB', 'TB'] as const

/** Formats a byte count with decimal units (1 GB = 1,000,000,000 B), e.g. "1.7 GB". */
export const formatBytes = (bytes: number): string => {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  let value = bytes
  let unit = 0
  while (value >= 1000 && unit < UNITS.length - 1) {
    value /= 1000
    unit += 1
  }
  return unit === 0 ? `${Math.round(value)} B` : `${value.toFixed(1)} ${UNITS[unit]}`
}
