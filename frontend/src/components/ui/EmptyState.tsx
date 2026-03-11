import * as React from 'react'

import { Button, type ButtonProps } from './Button'
import { cn } from '../../utils/cn'

export interface EmptyStateAction {
  label: string
  onClick: () => void
  variant?: ButtonProps['variant']
}

export interface EmptyStateProps extends React.HTMLAttributes<HTMLDivElement> {
  icon?: React.ReactNode
  title: string
  description: string
  action?: EmptyStateAction
  secondaryAction?: EmptyStateAction
}

/**
 * EmptyState - Empty state pattern component
 *
 * Features:
 * - Centered content with dashed border
 * - Icon in muted background circle
 * - Primary and secondary action buttons
 * - Uses Button component for actions
 */
export function EmptyState({
  icon,
  title,
  description,
  action,
  secondaryAction,
  className,
  ...props
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center py-16 px-6 text-center',
        'rounded-xl border border-dashed border-border bg-surface-subtle/50',
        className
      )}
      {...props}
    >
      {icon && (
        <div className="mb-4 rounded-full bg-surface-muted p-4 text-content-muted">
          {icon}
        </div>
      )}
      <h3 className="text-lg font-semibold text-content-primary">{title}</h3>
      <p className="mt-2 max-w-sm text-sm text-content-secondary">
        {description}
      </p>
      {(action || secondaryAction) && (
        <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
          {action && (
            <Button variant={action.variant || 'primary'} onClick={action.onClick}>
              {action.label}
            </Button>
          )}
          {secondaryAction && (
            <Button variant="ghost" onClick={secondaryAction.onClick}>
              {secondaryAction.label}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
