import { cn } from '../../utils/cn'

import type { StatsBucket } from '../../api/stats'

export interface BucketSelectorProps {
  value: StatsBucket
  onChange: (b: StatsBucket) => void
}

const BUCKETS: { label: string; value: StatsBucket }[] = [
  { label: '1h', value: '1h' },
  { label: '6h', value: '6h' },
  { label: '1d', value: '1d' },
]

export function BucketSelector({ value, onChange }: BucketSelectorProps) {
  return (
    <div
      role="group"
      aria-label="Select bucket granularity"
      className="inline-flex h-9 items-center rounded-lg bg-surface-subtle p-1 text-content-muted"
    >
      {BUCKETS.map((b) => (
        <button
          key={b.value}
          type="button"
          role="radio"
          aria-checked={value === b.value}
          onClick={() => onChange(b.value)}
          className={cn(
            'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium',
            'ring-offset-surface-base transition-all duration-fast',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
            value === b.value
              ? 'bg-surface-elevated text-content-primary shadow-sm'
              : 'hover:text-content-primary'
          )}
        >
          {b.label}
        </button>
      ))}
    </div>
  )
}
