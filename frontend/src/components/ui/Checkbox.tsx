import * as CheckboxPrimitive from '@radix-ui/react-checkbox'
import { Check, Minus } from 'lucide-react'
import * as React from 'react'

import { cn } from '../../utils/cn'

export interface CheckboxProps
  extends React.ComponentPropsWithoutRef<typeof CheckboxPrimitive.Root> {
  indeterminate?: boolean
}

const Checkbox = React.forwardRef<
  React.ElementRef<typeof CheckboxPrimitive.Root>,
  CheckboxProps
>(({ className, indeterminate, ...props }, ref) => (
  <CheckboxPrimitive.Root
    ref={ref}
    className={cn(
      'peer h-4 w-4 shrink-0 rounded',
      'border border-border',
      'bg-surface-base',
      'ring-offset-surface-base',
      'transition-colors duration-fast',
      'hover:border-brand-400',
      'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
      'disabled:cursor-not-allowed disabled:opacity-50',
      'data-[state=checked]:bg-brand-500 data-[state=checked]:border-brand-500 data-[state=checked]:text-white',
      'data-[state=indeterminate]:bg-brand-500 data-[state=indeterminate]:border-brand-500 data-[state=indeterminate]:text-white',
      className
    )}
    checked={indeterminate ? 'indeterminate' : props.checked}
    {...props}
  >
    <CheckboxPrimitive.Indicator
      className={cn('flex items-center justify-center text-current')}
    >
      {indeterminate ? (
        <Minus className="h-3 w-3" />
      ) : (
        <Check className="h-3 w-3" />
      )}
    </CheckboxPrimitive.Indicator>
  </CheckboxPrimitive.Root>
))
Checkbox.displayName = CheckboxPrimitive.Root.displayName

export { Checkbox }
