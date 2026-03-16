import * as ProgressPrimitive from '@radix-ui/react-progress'
import { cva, type VariantProps } from 'class-variance-authority'
import * as React from 'react'

import { cn } from '../../utils/cn'

const progressVariants = cva(
  'h-full w-full flex-1 transition-all duration-normal',
  {
    variants: {
      variant: {
        default: 'bg-brand-500',
        success: 'bg-success',
        warning: 'bg-warning',
        error: 'bg-error',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  }
)

export interface ProgressProps
  extends React.ComponentPropsWithoutRef<typeof ProgressPrimitive.Root>,
    VariantProps<typeof progressVariants> {
  showValue?: boolean
}

const Progress = React.forwardRef<
  React.ElementRef<typeof ProgressPrimitive.Root>,
  ProgressProps
>(({ className, value, variant, showValue = false, ...props }, ref) => (
  <div className="flex items-center gap-3">
    <ProgressPrimitive.Root
      ref={ref}
      className={cn(
        'relative h-2 w-full overflow-hidden rounded-full bg-surface-muted',
        className
      )}
      {...props}
    >
      <ProgressPrimitive.Indicator
        className={cn(progressVariants({ variant }), 'rounded-full')}
        style={{ transform: `translateX(-${100 - (value || 0)}%)` }}
      />
    </ProgressPrimitive.Root>
    {showValue && (
      <span className="text-sm font-medium text-content-secondary tabular-nums">
        {Math.round(value || 0)}%
      </span>
    )}
  </div>
))
Progress.displayName = ProgressPrimitive.Root.displayName

export { Progress }
