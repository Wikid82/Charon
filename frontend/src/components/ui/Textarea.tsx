import * as React from 'react'

import { cn } from '../../utils/cn'

export interface TextareaProps
  extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  error?: boolean
}

const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, error, ...props }, ref) => {
    return (
      <textarea
        className={cn(
          'flex min-h-[80px] w-full rounded-lg px-3 py-2',
          'border bg-surface-base text-content-primary',
          'text-sm placeholder:text-content-muted',
          'transition-colors duration-fast',
          error
            ? 'border-error focus:ring-error/20'
            : 'border-border hover:border-border-strong focus:border-brand-500',
          'focus:outline-none focus:ring-2 focus:ring-brand-500/20',
          'disabled:cursor-not-allowed disabled:opacity-50',
          'resize-y',
          className
        )}
        ref={ref}
        {...props}
      />
    )
  }
)
Textarea.displayName = 'Textarea'

export { Textarea }
