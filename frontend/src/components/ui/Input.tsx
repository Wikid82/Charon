import * as React from 'react'
import { Eye, EyeOff, type LucideIcon } from 'lucide-react'
import { cn } from '../../utils/cn'

export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  helperText?: string
  errorTestId?: string
  leftIcon?: LucideIcon
  rightIcon?: LucideIcon
}

const Input = React.forwardRef<HTMLInputElement, InputProps>(
  (
    {
      label,
      error,
      helperText,
      errorTestId,
      leftIcon: LeftIcon,
      rightIcon: RightIcon,
      className,
      type,
      disabled,
      ...props
    },
    ref
  ) => {
    const [showPassword, setShowPassword] = React.useState(false)
    const isPassword = type === 'password'

    return (
      <div className="w-full">
        {label && (
          <label
            htmlFor={props.id}
            className="block text-sm font-medium text-content-secondary mb-1.5"
          >
            {label}
          </label>
        )}
        <div className="relative">
          {LeftIcon && (
            <div className="absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none">
              <LeftIcon className="h-4 w-4 text-content-muted" />
            </div>
          )}
          <input
            ref={ref}
            type={isPassword ? (showPassword ? 'text' : 'password') : type}
            disabled={disabled}
            aria-describedby={error && errorTestId ? errorTestId : undefined}
            className={cn(
              'flex h-10 w-full rounded-lg px-4 py-2',
              'bg-surface-base border text-content-primary',
              'text-sm placeholder:text-content-muted',
              'transition-colors duration-fast',
              error
                ? 'border-error focus:ring-error/20'
                : 'border-border hover:border-border-strong focus:border-brand-500',
              'focus:outline-none focus:ring-2 focus:ring-brand-500/20',
              'disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:border-border',
              LeftIcon && 'pl-10',
              (isPassword || RightIcon) && 'pr-10',
              className
            )}
            {...props}
          />
          {isPassword && (
            <button
              type="button"
              onClick={() => setShowPassword(!showPassword)}
              className={cn(
                'absolute right-3 top-1/2 -translate-y-1/2',
                'text-content-muted hover:text-content-primary',
                'focus:outline-none transition-colors duration-fast'
              )}
              tabIndex={-1}
              aria-label={showPassword ? 'Hide password' : 'Show password'}
            >
              {showPassword ? (
                <EyeOff className="h-4 w-4" />
              ) : (
                <Eye className="h-4 w-4" />
              )}
            </button>
          )}
          {!isPassword && RightIcon && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2 pointer-events-none">
              <RightIcon className="h-4 w-4 text-content-muted" />
            </div>
          )}
        </div>
        {error && (
          <p
            id={errorTestId}
            className="mt-1.5 text-sm text-error"
            data-testid={errorTestId}
            role="alert"
          >
            {error}
          </p>
        )}
        {helperText && !error && (
          <p className="mt-1.5 text-sm text-content-muted">{helperText}</p>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'

export { Input }
