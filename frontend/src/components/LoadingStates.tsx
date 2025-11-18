export function LoadingSpinner({ size = 'md' }: { size?: 'sm' | 'md' | 'lg' }) {
  const sizeClasses = {
    sm: 'w-4 h-4 border-2',
    md: 'w-8 h-8 border-3',
    lg: 'w-12 h-12 border-4',
  }

  return (
    <div
      className={`${sizeClasses[size]} border-blue-600 border-t-transparent rounded-full animate-spin`}
      role="status"
      aria-label="Loading"
    />
  )
}

export function LoadingOverlay({ message = 'Loading...' }: { message?: string }) {
  return (
    <div className="fixed inset-0 bg-slate-900/50 backdrop-blur-sm flex items-center justify-center z-50">
      <div className="bg-slate-800 rounded-lg p-6 flex flex-col items-center gap-4 shadow-xl">
        <LoadingSpinner size="lg" />
        <p className="text-slate-300">{message}</p>
      </div>
    </div>
  )
}

export function LoadingCard() {
  return (
    <div className="bg-slate-800 rounded-lg p-6 animate-pulse">
      <div className="h-6 bg-slate-700 rounded w-1/3 mb-4"></div>
      <div className="space-y-3">
        <div className="h-4 bg-slate-700 rounded w-full"></div>
        <div className="h-4 bg-slate-700 rounded w-5/6"></div>
        <div className="h-4 bg-slate-700 rounded w-4/6"></div>
      </div>
    </div>
  )
}

export function EmptyState({
  icon = '📦',
  title,
  description,
  action,
}: {
  icon?: string
  title: string
  description: string
  action?: React.ReactNode
}) {
  return (
    <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
      <div className="text-6xl mb-4">{icon}</div>
      <h3 className="text-xl font-semibold text-slate-200 mb-2">{title}</h3>
      <p className="text-slate-400 mb-6 max-w-md">{description}</p>
      {action}
    </div>
  )
}
