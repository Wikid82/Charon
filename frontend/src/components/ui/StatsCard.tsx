import * as React from 'react'
import { TrendingUp, TrendingDown, Minus } from 'lucide-react'
import { cn } from '../../utils/cn'

export interface StatsCardChange {
  value: number
  trend: 'up' | 'down' | 'neutral'
  label?: string
}

export interface StatsCardProps {
  title: string
  value: string | number
  change?: StatsCardChange
  icon?: React.ReactNode
  href?: string
  className?: string
}

/**
 * StatsCard - KPI/metric card component
 *
 * Features:
 * - Trend indicators with TrendingUp/TrendingDown/Minus icons
 * - Color-coded trends (success for up, error for down, muted for neutral)
 * - Interactive hover state when href is provided
 * - Card styles (rounded-xl, border, shadow on hover)
 */
export function StatsCard({
  title,
  value,
  change,
  icon,
  href,
  className,
}: StatsCardProps) {
  const isInteractive = Boolean(href)

  const TrendIcon =
    change?.trend === 'up'
      ? TrendingUp
      : change?.trend === 'down'
        ? TrendingDown
        : Minus

  const trendColorClass =
    change?.trend === 'up'
      ? 'text-success'
      : change?.trend === 'down'
        ? 'text-error'
        : 'text-content-muted'

  const content = (
    <>
      <div className="flex items-start justify-between">
        <div className="min-w-0 flex-1">
          <p className="text-sm font-medium text-content-secondary truncate">
            {title}
          </p>
          <p className="mt-2 text-3xl font-bold text-content-primary tabular-nums">
            {value}
          </p>
          {change && (
            <div
              className={cn(
                'mt-2 flex items-center gap-1 text-sm',
                trendColorClass
              )}
            >
              <TrendIcon className="h-4 w-4 shrink-0" />
              <span className="font-medium">{change.value}%</span>
              {change.label && (
                <span className="text-content-muted truncate">
                  {change.label}
                </span>
              )}
            </div>
          )}
        </div>
        {icon && (
          <div className="shrink-0 rounded-lg bg-brand-500/10 p-3 text-brand-500">
            {icon}
          </div>
        )}
      </div>
    </>
  )

  const baseClasses = cn(
    'block rounded-xl border border-border bg-surface-elevated p-6',
    'transition-all duration-fast',
    isInteractive && [
      'hover:shadow-md hover:border-brand-500/50 cursor-pointer',
      'focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 focus-visible:ring-offset-surface-base',
    ],
    className
  )

  if (href) {
    return (
      <a href={href} className={baseClasses}>
        {content}
      </a>
    )
  }

  return <div className={baseClasses}>{content}</div>
}
