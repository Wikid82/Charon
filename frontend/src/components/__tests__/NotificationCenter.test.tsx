import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import NotificationCenter from '../NotificationCenter'
import * as api from '../../api/system'

// Mock the API
vi.mock('../../api/system', () => ({
  getNotifications: vi.fn(),
  markNotificationRead: vi.fn(),
  markAllNotificationsRead: vi.fn(),
  checkUpdates: vi.fn(),
}))

const createWrapper = () => {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
    },
  })
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

const mockNotifications: api.Notification[] = [
  {
    id: '1',
    type: 'info',
    title: 'Info Notification',
    message: 'This is an info message',
    read: false,
    created_at: '2025-01-01T10:00:00Z',
  },
  {
    id: '2',
    type: 'success',
    title: 'Success Notification',
    message: 'This is a success message',
    read: false,
    created_at: '2025-01-01T11:00:00Z',
  },
  {
    id: '3',
    type: 'warning',
    title: 'Warning Notification',
    message: 'This is a warning message',
    read: false,
    created_at: '2025-01-01T12:00:00Z',
  },
  {
    id: '4',
    type: 'error',
    title: 'Error Notification',
    message: 'This is an error message',
    read: false,
    created_at: '2025-01-01T13:00:00Z',
  },
]

describe('NotificationCenter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('renders bell icon and unread count', async () => {
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    render(<NotificationCenter />, { wrapper: createWrapper() })

    expect(screen.getByRole('button', { name: /notifications/i })).toBeInTheDocument()

    await waitFor(() => {
      expect(screen.getByText('4')).toBeInTheDocument()
    })
  })

  it('opens notification panel on click', async () => {
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    render(<NotificationCenter />, { wrapper: createWrapper() })

    const bellButton = screen.getByRole('button', { name: /notifications/i })
    fireEvent.click(bellButton)

    await waitFor(() => {
      expect(screen.getByText('Notifications')).toBeInTheDocument()
      expect(screen.getByText('Info Notification')).toBeInTheDocument()
      expect(screen.getByText('Success Notification')).toBeInTheDocument()
      expect(screen.getByText('Warning Notification')).toBeInTheDocument()
      expect(screen.getByText('Error Notification')).toBeInTheDocument()
    })
  })

  it('displays empty state when no notifications', async () => {
    vi.mocked(api.getNotifications).mockResolvedValue([])
    render(<NotificationCenter />, { wrapper: createWrapper() })

    const bellButton = screen.getByRole('button', { name: /notifications/i })
    fireEvent.click(bellButton)

    await waitFor(() => {
      expect(screen.getByText('No new notifications')).toBeInTheDocument()
    })
  })

  it('marks single notification as read', async () => {
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    vi.mocked(api.markNotificationRead).mockResolvedValue()

    render(<NotificationCenter />, { wrapper: createWrapper() })

    fireEvent.click(screen.getByRole('button', { name: /notifications/i }))

    await waitFor(() => {
      expect(screen.getByText('Info Notification')).toBeInTheDocument()
    })

    const closeButtons = screen.getAllByRole('button', { name: /close/i })
    fireEvent.click(closeButtons[0])

    await waitFor(() => {
      expect(api.markNotificationRead).toHaveBeenCalledWith('1', expect.anything())
    })
  })

  it('marks all notifications as read', async () => {
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    vi.mocked(api.markAllNotificationsRead).mockResolvedValue()

    render(<NotificationCenter />, { wrapper: createWrapper() })

    fireEvent.click(screen.getByRole('button', { name: /notifications/i }))

    await waitFor(() => {
      expect(screen.getByText('Mark all read')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByText('Mark all read'))

    await waitFor(() => {
      expect(api.markAllNotificationsRead).toHaveBeenCalled()
    })
  })

  it('closes panel when clicking outside', async () => {
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    render(<NotificationCenter />, { wrapper: createWrapper() })

    fireEvent.click(screen.getByRole('button', { name: /notifications/i }))

    await waitFor(() => {
      expect(screen.getByText('Notifications')).toBeInTheDocument()
    })

    fireEvent.click(screen.getByTestId('notification-backdrop'))

    await waitFor(() => {
      expect(screen.queryByText('Notifications')).not.toBeInTheDocument()
    })
  })
})
