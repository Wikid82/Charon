/**
 * Lightweight client-side validator for standard 5-field cron expressions
 * (minute hour day-of-month month day-of-week), matching the fields accepted
 * by the backend's `cron.ParseStandard` (robfig/cron/v3) at
 * `PUT /api/v1/backups/settings` (spec §3.3.2). This is intentionally a
 * light-weight syntactic check (not a full-fidelity reimplementation of
 * ParseStandard) used purely for inline UI feedback before the request is
 * sent — the backend remains the source of truth and is validated server-side
 * regardless of this check.
 */

interface FieldRange {
  min: number
  max: number
}

const FIELD_RANGES: FieldRange[] = [
  { min: 0, max: 59 }, // minute
  { min: 0, max: 23 }, // hour
  { min: 1, max: 31 }, // day of month
  { min: 1, max: 12 }, // month
  { min: 0, max: 7 }, // day of week (0 and 7 both mean Sunday)
]

function isValidCronField(field: string, range: FieldRange): boolean {
  if (field.length === 0) return false

  return field.split(',').every((part) => {
    const match = part.match(/^(\*|\d+(?:-\d+)?)(\/(\d+))?$/)
    if (!match) return false

    const [, base, , step] = match
    if (step !== undefined && Number(step) <= 0) return false
    if (base === '*') return true

    const bounds = base.split('-').map(Number)
    if (bounds.some((n) => Number.isNaN(n) || n < range.min || n > range.max)) return false
    if (bounds.length === 2 && bounds[0] > bounds[1]) return false
    return true
  })
}

/**
 * Returns true when `expression` looks like a syntactically valid 5-field
 * cron expression (supports `*`, ranges, lists, and `/step` syntax).
 */
export function isValidCronExpression(expression: string): boolean {
  const trimmed = expression.trim()
  if (!trimmed) return false

  const fields = trimmed.split(/\s+/)
  if (fields.length !== 5) return false

  return fields.every((field, i) => isValidCronField(field, FIELD_RANGES[i]))
}

/** UI-level schedule frequency preset (spec §3.8 — Daily/Weekly presets + a custom-cron escape hatch). */
export type ScheduleFrequency = 'daily' | 'weekly' | 'custom'

/**
 * Derives the {@link ScheduleFrequency} preset (and its time-of-day / day-of-week)
 * from a `schedule_cron` string, falling back to "custom" for anything that
 * doesn't match the simple daily/weekly shapes the presets themselves produce.
 */
export function parseCronPreset(cron: string): {
  frequency: ScheduleFrequency
  time: string
  dayOfWeek: string
} {
  const dailyMatch = cron.match(/^0 (\d{1,2}) \* \* \*$/)
  if (dailyMatch) {
    return { frequency: 'daily', time: `${dailyMatch[1].padStart(2, '0')}:00`, dayOfWeek: '0' }
  }
  const weeklyMatch = cron.match(/^0 (\d{1,2}) \* \* ([0-6])$/)
  if (weeklyMatch) {
    return { frequency: 'weekly', time: `${weeklyMatch[1].padStart(2, '0')}:00`, dayOfWeek: weeklyMatch[2] }
  }
  return { frequency: 'custom', time: '03:00', dayOfWeek: '0' }
}
