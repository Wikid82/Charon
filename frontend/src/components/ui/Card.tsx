import { ReactNode } from 'react'
import { clsx } from 'clsx'

interface CardProps {
  children: ReactNode
  className?: string
  title?: string
  description?: string
  footer?: ReactNode
}

export function Card({ children, className, title, description, footer }: CardProps) {
  return (
    <div className={clsx('bg-dark-card rounded-lg border border-gray-800 overflow-hidden', className)}>
      {(title || description) && (
        <div className="px-6 py-4 border-b border-gray-800">
          {title && <h3 className="text-lg font-medium text-white">{title}</h3>}
          {description && <p className="mt-1 text-sm text-gray-400">{description}</p>}
        </div>
      )}
      <div className="p-6">
        {children}
      </div>
      {footer && (
        <div className="px-6 py-4 bg-gray-900/50 border-t border-gray-800">
          {footer}
        </div>
      )}
    </div>
  )
}
