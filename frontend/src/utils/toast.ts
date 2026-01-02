type ToastType = 'success' | 'error' | 'info' | 'warning'

export interface Toast {
  id: number
  message: string
  type: ToastType
}

let toastId = 0
export const toastCallbacks = new Set<(toast: Toast) => void>()

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
