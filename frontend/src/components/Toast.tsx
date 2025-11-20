import { useEffect, useState } from 'react'

type ToastType = 'success' | 'error' | 'info' | 'warning'

interface Toast {
  id: number
  message: string
  type: ToastType
}

let toastId = 0
const toastCallbacks = new Set<(toast: Toast) => void>()

export const toast = {
  success: (message: string) => {
    const id = ++toastId
    toastCallbacks.forEach(callback => callback({ id, message, type: 'success' }))
  },
  error: (message: string) => {
    const id = ++toastId
    toastCallbacks.forEach(callback => callback({ id, message, type: 'error' }))
  },
  info: (message: string) => {
    const id = ++toastId
    toastCallbacks.forEach(callback => callback({ id, message, type: 'info' }))
  },
  warning: (message: string) => {
    const id = ++toastId
    toastCallbacks.forEach(callback => callback({ id, message, type: 'warning' }))
  },
}

export function ToastContainer() {
  const [toasts, setToasts] = useState<Toast[]>([])

  useEffect(() => {
    const callback = (toast: Toast) => {
      setToasts(prev => [...prev, toast])
      setTimeout(() => {
        setToasts(prev => prev.filter(t => t.id !== toast.id))
      }, 5000)
    }
    toastCallbacks.add(callback)
    return () => {
      toastCallbacks.delete(callback)
    }
  }, [])

  const removeToast = (id: number) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }

  return (
    <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
      {toasts.map(toast => (
        <div
          key={toast.id}
          className={`pointer-events-auto px-4 py-3 rounded-lg shadow-lg flex items-center gap-3 min-w-[300px] max-w-[500px] animate-slide-in ${
            toast.type === 'success'
              ? 'bg-green-600 text-white'
              : toast.type === 'error'
              ? 'bg-red-600 text-white'
              : toast.type === 'warning'
              ? 'bg-yellow-600 text-white'
              : 'bg-blue-600 text-white'
          }`}
        >
          <div className="flex-1">
            {toast.type === 'success' && <span className="mr-2">✓</span>}
            {toast.type === 'error' && <span className="mr-2">✗</span>}
            {toast.type === 'warning' && <span className="mr-2">⚠</span>}
            {toast.type === 'info' && <span className="mr-2">ℹ</span>}
            {toast.message}
          </div>
          <button
            onClick={() => removeToast(toast.id)}
            className="text-white/80 hover:text-white transition-colors"
            aria-label="Close"
          >
            ×
          </button>
        </div>
      ))}
    </div>
  )
}
