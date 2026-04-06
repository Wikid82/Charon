import { useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import type { TimeRange } from '../../api/crowdsecDashboard'

interface DashboardTimeRangeSelectorProps {
  value: TimeRange
  onChange: (range: TimeRange) => void
}

const RANGES: TimeRange[] = ['1h', '6h', '24h', '7d', '30d']

const RANGE_LABELS: Record<TimeRange, string> = {
  '1h': '1H',
  '6h': '6H',
  '24h': '24H',
  '7d': '7D',
  '30d': '30D',
}

export function DashboardTimeRangeSelector({ value, onChange }: DashboardTimeRangeSelectorProps) {
  const { t } = useTranslation()
  const buttonRefs = useRef<(HTMLButtonElement | null)[]>([])

  const selectedIndex = RANGES.indexOf(value)

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLButtonElement>) => {
      let nextIndex: number

      switch (e.key) {
        case 'ArrowRight':
        case 'ArrowDown':
          nextIndex = (selectedIndex + 1) % RANGES.length
          break
        case 'ArrowLeft':
        case 'ArrowUp':
          nextIndex = (selectedIndex - 1 + RANGES.length) % RANGES.length
          break
        case 'Home':
          nextIndex = 0
          break
        case 'End':
          nextIndex = RANGES.length - 1
          break
        default:
          return
      }

      e.preventDefault()
      onChange(RANGES[nextIndex])
      buttonRefs.current[nextIndex]?.focus()
    },
    [selectedIndex, onChange],
  )

  return (
    <div
      role="radiogroup"
      aria-label={t('security.crowdsec.dashboard.timeRange', 'Time range')}
      className="inline-flex rounded-lg border border-gray-700 bg-gray-900 p-1"
    >
      {RANGES.map((range, i) => (
        <button
          key={range}
          ref={(el) => { buttonRefs.current[i] = el }}
          role="radio"
          aria-checked={value === range}
          tabIndex={value === range ? 0 : -1}
          onClick={() => onChange(range)}
          onKeyDown={handleKeyDown}
          className={`px-3 py-1.5 text-sm font-medium rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 ${
            value === range
              ? 'bg-blue-600 text-white'
              : 'text-gray-400 hover:text-white hover:bg-gray-800'
          }`}
        >
          {RANGE_LABELS[range]}
        </button>
      ))}
    </div>
  )
}
