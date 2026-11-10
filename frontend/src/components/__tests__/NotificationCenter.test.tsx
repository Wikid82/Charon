import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

import * as api from '../../api/system'
import { ALERT_PRESENTATION } from '../../constants/alerts'
import { GEOIP_TROUBLESHOOTING_DOCS_URL } from '../../constants/docs'
import { GITHUB_BUG_REPORT_URL } from '../../constants/links'
import NotificationCenter from '../NotificationCenter'

// Mock the API
vi.mock('../../api/system', () => ({
  getNotifications: vi.fn(),
  markNotificationRead: vi.fn(),
  markAllNotificationsRead: vi.fn(),
  checkUpdates: vi.fn(),
  getActiveAlerts: vi.fn(),
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

const geoipAlert: api.ActiveAlert = {
  key: 'geoip_unavailable',
  severity: 'error',
  affected_lists: ['No RU/CN', 'Office only'],
  affected_lists_total: 2,
  database_available: false,
}

const GEOIP_TITLE = 'Country access lists are not being enforced'
const GEOIP_MESSAGE =
  'The GeoIP country database is missing or unusable, so 2 country list(s) (No RU/CN and Office only) let every visitor through until it is available again.'

describe('NotificationCenter', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(api.checkUpdates).mockResolvedValue({
      current_version: '1.0.0',
      available: false,
      latest_version: '0.0.0',
      changelog_url: '',
    })
    vi.mocked(api.getActiveAlerts).mockResolvedValue([])
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
    const user = userEvent.setup()
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    render(<NotificationCenter />, { wrapper: createWrapper() })

    const bellButton = screen.getByRole('button', { name: /notifications/i })
    await user.click(bellButton)

    await waitFor(() => {
      expect(screen.getByText('Notifications')).toBeInTheDocument()
      expect(screen.getByText('Info Notification')).toBeInTheDocument()
      expect(screen.getByText('Success Notification')).toBeInTheDocument()
      expect(screen.getByText('Warning Notification')).toBeInTheDocument()
      expect(screen.getByText('Error Notification')).toBeInTheDocument()
    })
  })

  it('displays empty state when no notifications', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getNotifications).mockResolvedValue([])
    render(<NotificationCenter />, { wrapper: createWrapper() })

    const bellButton = screen.getByRole('button', { name: /notifications/i })
    await user.click(bellButton)

    await waitFor(() => {
      expect(screen.getByText('No new notifications')).toBeInTheDocument()
    })
  })

  it('marks single notification as read', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    vi.mocked(api.markNotificationRead).mockResolvedValue()

    render(<NotificationCenter />, { wrapper: createWrapper() })

    await user.click(screen.getByRole('button', { name: /notifications/i }))

    await waitFor(() => {
      expect(screen.getByText('Info Notification')).toBeInTheDocument()
    })

    const closeButtons = screen.getAllByRole('button', { name: 'Dismiss' })
    await user.click(closeButtons[0])

    await waitFor(() => {
      expect(api.markNotificationRead).toHaveBeenCalledWith('1', expect.anything())
    })
  })

  it('marks all notifications as read', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    vi.mocked(api.markAllNotificationsRead).mockResolvedValue()

    render(<NotificationCenter />, { wrapper: createWrapper() })

    await user.click(screen.getByRole('button', { name: /notifications/i }))

    await waitFor(() => {
      expect(screen.getByText('Mark all read')).toBeInTheDocument()
    })

    await user.click(screen.getByText('Mark all read'))

    await waitFor(() => {
      expect(api.markAllNotificationsRead).toHaveBeenCalled()
    })
  })

  it('closes panel when clicking outside', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
    render(<NotificationCenter />, { wrapper: createWrapper() })

    await user.click(screen.getByRole('button', { name: /notifications/i }))

    await waitFor(() => {
      expect(screen.getByText('Notifications')).toBeInTheDocument()
    })

    await user.click(screen.getByTestId('notification-backdrop'))

    await waitFor(() => {
      expect(screen.queryByText('Notifications')).not.toBeInTheDocument()
    })
  })

  it('shows the update notification with a changelog link when one is available', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getNotifications).mockResolvedValue([])
    vi.mocked(api.checkUpdates).mockResolvedValue({
      current_version: '0.46.5',
      available: true,
      latest_version: '0.46.6',
      changelog_url: 'https://github.com/Wikid82/Charon/releases/tag/v0.46.6',
    })
    render(<NotificationCenter />, { wrapper: createWrapper() })

    await user.click(screen.getByRole('button', { name: /notifications/i }))

    const link = await screen.findByRole('link', { name: /view changelog/i })
    expect(link).toHaveAttribute('href', 'https://github.com/Wikid82/Charon/releases/tag/v0.46.6')
    expect(screen.getByText('Update available: 0.46.6')).toBeInTheDocument()
  })

  it('hides the changelog link when the update has no changelog_url', async () => {
    const user = userEvent.setup()
    vi.mocked(api.getNotifications).mockResolvedValue([])
    vi.mocked(api.checkUpdates).mockResolvedValue({
      current_version: '0.46.5',
      available: true,
      latest_version: '0.46.6',
      changelog_url: '',
    })
    render(<NotificationCenter />, { wrapper: createWrapper() })

    await user.click(screen.getByRole('button', { name: /notifications/i }))

    await screen.findByText('Update available: 0.46.6')
    expect(screen.queryByRole('link', { name: /view changelog/i })).toBeNull()
  })

  describe('active alerts', () => {
    const openBell = async (user: ReturnType<typeof userEvent.setup>) => {
      await user.click(screen.getByRole('button', { name: /notifications/i }))
    }

    it('shows an error alert first, in a labelled region, with its title and body', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
      vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)

      const region = await screen.findByRole('region', { name: 'Needs attention' })
      expect(region).toHaveTextContent(GEOIP_TITLE)
      expect(region).toHaveTextContent(GEOIP_MESSAGE)
      expect(region).toHaveTextContent('This clears by itself when the problem is fixed.')
      const firstNotification = screen.getByText('Info Notification')
      expect(region.compareDocumentPosition(firstNotification) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
    })

    it('turns the bell red and counts the alert in the badge and the accessible label', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
      vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      const bell = await screen.findByRole('button', { name: 'Notifications, 5 new' })
      await waitFor(() => expect(within(bell).getByText('5')).toBeInTheDocument())
      expect(bell).toHaveClass('text-red-500')

      await user.click(bell)
      expect(await screen.findByRole('region', { name: 'Needs attention' })).toBeInTheDocument()
    })

    it('labels the bell with the alert alone when nothing else is unread', async () => {
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      const bell = await screen.findByRole('button', { name: 'Notifications, 1 new' })
      await waitFor(() => expect(within(bell).getByText('1')).toBeInTheDocument())
      expect(bell).toHaveClass('text-red-500')
    })

    it('links to the GeoIP docs page in a new tab and to the bug report form', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)
      const region = await screen.findByRole('region', { name: 'Needs attention' })

      const docsLink = within(region).getByRole('link', { name: /How to provide the database/ })
      expect(docsLink).toHaveAttribute('href', GEOIP_TROUBLESHOOTING_DOCS_URL)
      expect(docsLink).toHaveAttribute('target', '_blank')
      expect(docsLink).toHaveAttribute('rel', 'noopener noreferrer')

      const issueLink = within(region).getByRole('link', { name: /Open an issue/ })
      expect(issueLink).toHaveAttribute('href', GITHUB_BUG_REPORT_URL)
      expect(issueLink).toHaveAttribute('target', '_blank')
      expect(issueLink).toHaveAttribute('rel', 'noopener noreferrer')
    })

    it('offers no dismiss control on an alert and leaves it in place after Mark all read', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue(mockNotifications)
      vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
      vi.mocked(api.markAllNotificationsRead).mockResolvedValue()
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)
      const region = await screen.findByRole('region', { name: 'Needs attention' })
      expect(within(region).queryByRole('button')).toBeNull()

      await user.click(screen.getByRole('button', { name: 'Mark all read' }))

      await waitFor(() => expect(api.markAllNotificationsRead).toHaveBeenCalled())
      expect(screen.getByRole('region', { name: 'Needs attention' })).toHaveTextContent(GEOIP_TITLE)
    })

    it('uses the pending-apply message once a database is available again', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([{ ...geoipAlert, database_available: true }])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)

      const region = await screen.findByRole('region', { name: 'Needs attention' })
      expect(region).toHaveTextContent(
        'The GeoIP database is available again, but 2 country list(s) (No RU/CN and Office only) are not enforced yet. Reload the GeoIP database from the Security page or restart Charon.',
      )
      expect(region).not.toHaveTextContent('is missing or unusable')
    })

    it('renders list names as plain text, never as markup', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([
        { ...geoipAlert, affected_lists: ['<b>bold</b>'], affected_lists_total: 1 },
      ])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)

      const region = await screen.findByRole('region', { name: 'Needs attention' })
      expect(region).toHaveTextContent('(<b>bold</b>)')
      expect(screen.queryByText('bold', { selector: 'b' })).toBeNull()
    })

    it('joins affected list names with a conjunction and counts the full total', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([
        {
          ...geoipAlert,
          affected_lists: ['Alpha', 'Beta', 'Gamma'],
          affected_lists_total: 7,
        },
      ])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)

      const region = await screen.findByRole('region', { name: 'Needs attention' })
      expect(region).toHaveTextContent('so 7 country list(s) (Alpha, Beta, and Gamma)')
    })

    it('renders an alert without affected lists as an empty list, not an error', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([{ key: 'geoip_unavailable', severity: 'error' }])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)

      const region = await screen.findByRole('region', { name: 'Needs attention' })
      expect(region).toHaveTextContent('so 0 country list(s) () let every visitor')
    })

    it('does not render or count an alert with an unknown key and logs nothing', async () => {
      const user = userEvent.setup()
      const consoleSpies = [vi.spyOn(console, 'log'), vi.spyOn(console, 'warn'), vi.spyOn(console, 'error')]
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([{ key: 'future_alert', severity: 'error' }])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await waitFor(() => expect(api.getActiveAlerts).toHaveBeenCalled())
      expect(screen.getByRole('button', { name: 'Notifications' })).not.toHaveClass('text-red-500')
      expect(screen.queryByText('future_alert')).toBeNull()

      await openBell(user)

      expect(screen.queryByRole('region', { name: 'Needs attention' })).toBeNull()
      expect(screen.getByText('No new notifications')).toBeInTheDocument()
      consoleSpies.forEach((spy) => expect(spy).not.toHaveBeenCalled())
      consoleSpies.forEach((spy) => spy.mockRestore())
    })

    it('shows the alert and the update together, counting both', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
      vi.mocked(api.checkUpdates).mockResolvedValue({
        current_version: '0.46.5',
        available: true,
        latest_version: '0.46.6',
        changelog_url: 'https://github.com/Wikid82/Charon/releases/tag/v0.46.6',
      })
      render(<NotificationCenter />, { wrapper: createWrapper() })

      const bell = await screen.findByRole('button', { name: 'Notifications, 2 new' })
      await waitFor(() => expect(within(bell).getByText('2')).toBeInTheDocument())

      await user.click(bell)

      expect(await screen.findByRole('region', { name: 'Needs attention' })).toBeInTheDocument()
      expect(screen.getByText('Update available: 0.46.6')).toBeInTheDocument()
    })

    it('turns the bell yellow, not red, for a warning-severity registered alert', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([{ ...geoipAlert, severity: 'warning' }])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      const bell = await screen.findByRole('button', { name: 'Notifications, 1 new' })
      expect(bell).not.toHaveClass('text-red-500')
      expect(bell).toHaveClass('text-yellow-500')

      await user.click(bell)
      expect(await screen.findByRole('region', { name: 'Needs attention' })).toHaveTextContent(GEOIP_TITLE)
    })

    it('omits the issue link for a registered alert that does not ask for one', async () => {
      const user = userEvent.setup()
      const presentation = ALERT_PRESENTATION.geoip_unavailable
      const originalShowIssueLink = presentation.showIssueLink
      Object.assign(presentation, { showIssueLink: false })
      try {
        vi.mocked(api.getNotifications).mockResolvedValue([])
        vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
        render(<NotificationCenter />, { wrapper: createWrapper() })

        await openBell(user)

        const region = await screen.findByRole('region', { name: 'Needs attention' })
        expect(within(region).getByRole('link', { name: /How to provide the database/ })).toBeInTheDocument()
        expect(within(region).queryByRole('link', { name: /Open an issue/ })).toBeNull()
      } finally {
        Object.assign(presentation, { showIssueLink: originalShowIssueLink })
      }
    })

    it('hides the Mark all read control and the empty state when only an alert is present', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      vi.mocked(api.getActiveAlerts).mockResolvedValue([geoipAlert])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      await openBell(user)

      await screen.findByRole('region', { name: 'Needs attention' })
      expect(screen.queryByRole('button', { name: 'Mark all read' })).toBeNull()
      expect(screen.queryByText('No new notifications')).toBeNull()
    })

    it('keeps the plain bell label and empty state when there is nothing to show', async () => {
      const user = userEvent.setup()
      vi.mocked(api.getNotifications).mockResolvedValue([])
      render(<NotificationCenter />, { wrapper: createWrapper() })

      const bell = await screen.findByRole('button', { name: 'Notifications' })
      expect(bell).toHaveClass('text-green-500')
      expect(within(bell).queryByText(/^\d+$/)).toBeNull()

      await user.click(bell)
      expect(screen.getByText('No new notifications')).toBeInTheDocument()
    })
  })
})
