import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { Loader2, type LucideIcon } from 'lucide-react'
import { cn } from '../../utils/cn'

const buttonVariants = cva(
  [
    'inline-flex items-center justify-center gap-2',
    'rounded-lg font-medium',
    'transition-all duration-fast',
    'focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-offset-surface-base',
    'disabled:opacity-50 disabled:cursor-not-allowed disabled:pointer-events-none',
  ],
  {
    variants: {
      variant: {
        primary: [
          'bg-brand-500 text-white',
          'hover:bg-brand-600',
          'focus-visible:ring-brand-500',
          'active:bg-brand-700',
        ],
        secondary: [
          'bg-surface-muted text-content-primary',
          'hover:bg-surface-subtle',
          'focus-visible:ring-content-muted',
          'active:bg-surface-base',
        ],
        danger: [
          'bg-error text-white',
          'hover:bg-error/90',
          'focus-visible:ring-error',
          'active:bg-error/80',
        ],
        ghost: [
          'text-content-secondary bg-transparent',
          'hover:bg-surface-muted hover:text-content-primary',
          'focus-visible:ring-content-muted',
        ],
        outline: [
          'border border-border bg-transparent text-content-primary',
          'hover:bg-surface-subtle hover:border-border-strong',
          'focus-visible:ring-brand-500',
        ],
        link: [
          'text-brand-500 bg-transparent underline-offset-4',
          'hover:underline hover:text-brand-400',
          'focus-visible:ring-brand-500',
          'p-0 h-auto',
        ],
      },
      size: {
        sm: 'h-8 px-3 text-sm',
        md: 'h-10 px-4 text-sm',
        lg: 'h-12 px-6 text-base',
        icon: 'h-10 w-10 p-0',
      },
    },
    defaultVariants: {
      variant: 'primary',
      size: 'md',
    },
  }
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  isLoading?: boolean
  leftIcon?: LucideIcon
  rightIcon?: LucideIcon
  asChild?: boolean
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant,
      size,
      isLoading = false,
      leftIcon: LeftIcon,
      rightIcon: RightIcon,
      disabled,
      children,
      ...props
    },
    ref
  ) => {
    return (
      <button
        className={cn(buttonVariants({ variant, size }), className)}
        ref={ref}
        disabled={disabled || isLoading}
        {...props}
      >
        {isLoading ? (
          <Loader2 className="h-4 w-4 animate-spin" />
        ) : (
          LeftIcon && <LeftIcon className="h-4 w-4" />
        )}
        {children}
        {!isLoading && RightIcon && <RightIcon className="h-4 w-4" />}
      </button>
    )
  }
)
Button.displayName = 'Button'

// eslint-disable-next-line react-refresh/only-export-components
export { Button, buttonVariants }
