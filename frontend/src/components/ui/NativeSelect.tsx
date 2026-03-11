import { forwardRef } from 'react';

import { cn } from '../../utils/cn';

export interface NativeSelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  error?: boolean;
}

export const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(
  ({ className, error, ...props }, ref) => {
    return (
      <select
        ref={ref}
        className={cn(
          'flex h-10 w-full items-center justify-between gap-2',
          'rounded-lg border px-3 py-2',
          'bg-surface-base text-content-primary text-sm',
          'placeholder:text-content-muted',
          'transition-colors duration-fast',
          error
            ? 'border-error focus:ring-error'
            : 'border-border hover:border-border-strong focus:border-brand-500',
          'focus:outline-none focus:ring-2 focus:ring-brand-500/20',
          'disabled:cursor-not-allowed disabled:opacity-50',
          className
        )}
        {...props}
      />
    );
  }
);

NativeSelect.displayName = 'NativeSelect';
