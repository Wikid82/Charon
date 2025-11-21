import { InputHTMLAttributes, forwardRef, useState } from 'react'
import { clsx } from 'clsx'
import { Eye, EyeOff } from 'lucide-react'

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string
  error?: string
  helperText?: string
}

export const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ label, error, helperText, className, type, ...props }, ref) => {
    const [showPassword, setShowPassword] = useState(false)
    const isPassword = type === 'password'

    return (
      <div className="w-full">
        {label && (
          <label
            htmlFor={props.id}
            className="block text-sm font-medium text-gray-300 mb-1.5"
          >
            {label}
          </label>
        )}
        <div className="relative">
          <input
            ref={ref}
            type={isPassword ? (showPassword ? 'text' : 'password') : type}
            className={clsx(
              'w-full bg-gray-900 border rounded-lg px-4 py-2 text-white placeholder-gray-500 focus:outline-none focus:ring-2 transition-colors',
              error
                ? 'border-red-500 focus:ring-red-500'
                : 'border-gray-700 focus:ring-blue-500 focus:border-blue-500',
              isPassword && 'pr-10',
              className
            )}
            {...props}
          />
          {isPassword && (
            <button
              type="button"
              onClick={() => setShowPassword(!showPassword)}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-gray-500 hover:text-gray-300 focus:outline-none"
              tabIndex={-1}
            >
              {showPassword ? (
                <EyeOff className="h-4 w-4" />
              ) : (
                <Eye className="h-4 w-4" />
              )}
            </button>
          )}
        </div>
        {error && (
          <p className="mt-1 text-sm text-red-400">{error}</p>
        )}
        {helperText && !error && (
          <p className="mt-1 text-sm text-gray-500">{helperText}</p>
        )}
      </div>
    )
  }
)

Input.displayName = 'Input'
