import { cn } from '../../utils/cn'

import type { StatsPeriod } from '../../api/stats'

export interface PeriodSelectorProps {
  value: StatsPeriod
  onChange: (p: StatsPeriod) => void
}

const PERIODS: { label: string; value: StatsPeriod }[] = [
  { label: '24h', value: '24h' },
  { label: '7d', value: '7d' },
  { label: '30d', value: '30d' },
]

export function PeriodSelector({ value, onChange }: PeriodSelectorProps) {
  return (
    <div
      role="group"
      aria-label="Select time period"
      className="inline-flex h-9 items-center rounded-lg bg-surface-subtle p-1 text-content-muted"
    >
      {PERIODS.map((p) => (
        <button
          key={p.value}
          type="button"
          role="radio"
          aria-checked={value === p.value}
          onClick={() => onChange(p.value)}
          className={cn(
            'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium',
            'ring-offset-surface-base transition-all duration-fast',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
            value === p.value
              ? 'bg-surface-elevated text-content-primary shadow-sm'
              : 'hover:text-content-primary'
          )}
        >
          {p.label}
        </button>
      ))}
    </div>
  )
}
