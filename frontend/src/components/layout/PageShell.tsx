import * as React from 'react'

import { cn } from '../../utils/cn'

export interface PageShellProps {
  title: string
  description?: string
  actions?: React.ReactNode
  children: React.ReactNode
  className?: string
}

/**
 * PageShell - Consistent page wrapper component
 *
 * Provides standardized page layout with:
 * - Title (h1, text-2xl font-bold)
 * - Optional description (text-sm text-content-secondary)
 * - Optional actions slot for buttons
 * - Responsive flex layout (column on mobile, row on desktop)
 * - Consistent page spacing
 */
export function PageShell({
  title,
  description,
  actions,
  children,
  className,
}: PageShellProps) {
  return (
    <div className={cn('space-y-6', className)}>
      <header className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0 flex-1">
          <h1 className="text-2xl font-bold text-content-primary truncate">
            {title}
          </h1>
          {description && (
            <p className="mt-1 text-sm text-content-secondary">{description}</p>
          )}
        </div>
        {actions && (
          <div className="flex shrink-0 items-center gap-3">{actions}</div>
        )}
      </header>
      {children}
    </div>
  )
}
