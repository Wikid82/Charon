import { cva, type VariantProps } from 'class-variance-authority'

import { cn } from '../../utils/cn'

const badgeVariants = cva(
  'inline-flex items-center justify-center font-medium transition-colors duration-fast',
  {
    variants: {
      variant: {
        default: 'bg-surface-muted text-content-primary border border-border',
        primary: 'bg-brand-500 text-white',
        success: 'bg-success text-white',
        warning: 'bg-warning text-content-inverted',
        destructive: 'bg-error text-white',
        error: 'bg-error text-white',
        secondary: 'bg-surface-muted text-content-secondary border border-border',
        outline: 'border border-border text-content-secondary bg-transparent',
      },
      size: {
        sm: 'text-xs px-2 py-0.5 rounded',
        md: 'text-sm px-2.5 py-0.5 rounded-md',
        lg: 'text-base px-3 py-1 rounded-lg',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'md',
    },
  }
)

export interface BadgeProps
  extends React.HTMLAttributes<HTMLSpanElement>,
    VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, size, ...props }: BadgeProps) {
  return (
    <span
      className={cn(badgeVariants({ variant, size }), className)}
      {...props}
    />
  )
}
