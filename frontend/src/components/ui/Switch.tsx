import * as React from 'react'
import { cn } from '../../utils/cn'

interface SwitchProps extends React.InputHTMLAttributes<HTMLInputElement> {
  onCheckedChange?: (checked: boolean) => void
}

const Switch = React.forwardRef<HTMLInputElement, SwitchProps>(
  ({ className, onCheckedChange, onChange, id, disabled, ...props }, ref) => {
    return (
      <label
        htmlFor={id}
        className={cn(
          'relative inline-flex items-center',
          disabled ? 'cursor-not-allowed opacity-50' : 'cursor-pointer',
          className
        )}
      >
        <input
          id={id}
          type="checkbox"
          className="sr-only peer"
          ref={ref}
          disabled={disabled}
          onChange={(e) => {
            onChange?.(e)
            onCheckedChange?.(e.target.checked)
          }}
          {...props}
        />
        <div
          className={cn(
            'w-11 h-6 rounded-full transition-colors duration-fast',
            'bg-surface-muted',
            'peer-focus-visible:outline-none peer-focus-visible:ring-2 peer-focus-visible:ring-brand-500 peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-surface-base',
            'peer-checked:bg-brand-500',
            "after:content-[''] after:absolute after:top-[2px] after:start-[2px]",
            'after:bg-white after:border after:border-border after:rounded-full',
            'after:h-5 after:w-5 after:transition-all after:duration-fast',
            'peer-checked:after:translate-x-full peer-checked:after:border-white',
            'rtl:peer-checked:after:-translate-x-full'
          )}
        />
      </label>
    )
  }
)
Switch.displayName = 'Switch'

export { Switch }
