import { cva, type VariantProps } from 'class-variance-authority'
import {
  Info,
  CheckCircle,
  AlertTriangle,
  XCircle,
  X,
  type LucideIcon,
} from 'lucide-react'
import * as React from 'react'

import { cn } from '../../utils/cn'


const alertVariants = cva(
  'relative flex gap-3 p-4 rounded-lg border transition-all duration-normal',
  {
    variants: {
      variant: {
        default: 'bg-surface-subtle border-border text-content-primary',
        info: 'bg-info-muted border-info/30 text-content-primary',
        success: 'bg-success-muted border-success/30 text-content-primary',
        warning: 'bg-warning-muted border-warning/30 text-content-primary',
        error: 'bg-error-muted border-error/30 text-content-primary',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

const iconMap: Record<string, LucideIcon> = {
  default: Info,
  info: Info,
  success: CheckCircle,
  warning: AlertTriangle,
  error: XCircle,
}

const iconColorMap: Record<string, string> = {
  default: 'text-content-muted',
  info: 'text-info',
  success: 'text-success',
  warning: 'text-warning',
  error: 'text-error',
}

export interface AlertProps
  extends React.HTMLAttributes<HTMLDivElement>,
    VariantProps<typeof alertVariants> {
  title?: string
  icon?: LucideIcon
  dismissible?: boolean
  onDismiss?: () => void
}

export function Alert({
  className,
  variant = 'default',
  title,
  icon,
  dismissible = false,
  onDismiss,
  children,
  ...props
}: AlertProps) {
  const [isVisible, setIsVisible] = React.useState(true)

  if (!isVisible) return null

  const IconComponent = icon || iconMap[variant || 'default']
  const iconColor = iconColorMap[variant || 'default']

  const handleDismiss = () => {
    setIsVisible(false)
    onDismiss?.()
  }

  return (
    <div
      role="alert"
      className={cn(alertVariants({ variant }), className)}
      {...props}
    >
      <IconComponent className={cn('h-5 w-5 shrink-0 mt-0.5', iconColor)} />
      <div className="flex-1 min-w-0">
        {title && (
          <h5 className="font-semibold text-sm mb-1">{title}</h5>
        )}
        <div className="text-sm text-content-secondary">{children}</div>
      </div>
      {dismissible && (
        <button
          type="button"
          onClick={handleDismiss}
          className="shrink-0 p-1 rounded-md text-content-muted hover:text-content-primary hover:bg-surface-muted transition-colors duration-fast"
          aria-label="Dismiss alert"
        >
          <X className="h-4 w-4" />
        </button>
      )}
    </div>
  )
}

export type AlertTitleProps = React.HTMLAttributes<HTMLHeadingElement>

export function AlertTitle({ className, ...props }: AlertTitleProps) {
  return (
    <h5
      className={cn('font-semibold text-sm mb-1', className)}
      {...props}
    />
  )
}

export type AlertDescriptionProps = React.HTMLAttributes<HTMLParagraphElement>

export function AlertDescription({ className, ...props }: AlertDescriptionProps) {
  return (
    <p
      className={cn('text-sm text-content-secondary', className)}
      {...props}
    />
  )
}
