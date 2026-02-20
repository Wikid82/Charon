/**
 * Tests for security notification settings on the Notifications page.
 * The modal has been removed; settings are now managed on /settings/notifications.
 */
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import Notifications from '../../pages/Notifications';
import { createTestQueryClient } from '../../test/createTestQueryClient';
import * as notificationsApi from '../../api/notifications';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock('../../api/notifications', async () => {
  const actual = await vi.importActual('../../api/notifications');
  return {
    ...actual,
    getProviders: vi.fn(),
    getTemplates: vi.fn(),
    getExternalTemplates: vi.fn(),
    getSecurityNotificationSettings: vi.fn(),
    updateSecurityNotificationSettings: vi.fn(),
  };
});

vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

const mockSettings: notificationsApi.SecurityNotificationSettings = {
  enabled: true,
  min_log_level: 'warn',
  security_waf_enabled: true,
  security_acl_enabled: true,
  security_rate_limit_enabled: false,
  destination_ambiguous: false,
  webhook_url: 'https://example.com/webhook',
};

describe('Security Notification Settings on Notifications page', () => {
  let queryClient: ReturnType<typeof createTestQueryClient>;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();
    vi.mocked(notificationsApi.getProviders).mockResolvedValue([]);
    vi.mocked(notificationsApi.getTemplates).mockResolvedValue([]);
    vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([]);
    vi.mocked(notificationsApi.getSecurityNotificationSettings).mockResolvedValue(mockSettings);
    vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockResolvedValue(mockSettings);
  });

  const renderPage = () =>
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <Notifications />
        </MemoryRouter>
      </QueryClientProvider>
    );

  it('renders the security notifications section', async () => {
    renderPage();
    expect(await screen.findByTestId('security-notifications-section')).toBeInTheDocument();
  });

  it('loads and displays existing compatibility security settings', async () => {
    renderPage();

    await waitFor(() => {
      const enableSwitch = screen.getByTestId('security-notifications-enabled') as HTMLInputElement;
      expect(enableSwitch.checked).toBe(true);
    });

    const webhookInput = screen.getByTestId('security-webhook-url') as HTMLInputElement;
    expect(webhookInput.value).toBe('https://example.com/webhook');
    expect(screen.getByTestId('security-compatibility-banner')).toBeInTheDocument();
  });

  it('shows compatibility controls as read-only', async () => {
    vi.mocked(notificationsApi.getSecurityNotificationSettings).mockResolvedValue({
      ...mockSettings,
      enabled: false,
    });

    renderPage();

    await waitFor(() => {
      const enableSwitch = screen.getByTestId('security-notifications-enabled') as HTMLInputElement;
      expect(enableSwitch.checked).toBe(false);
    });

    expect((screen.getByTestId('security-min-log-level') as HTMLSelectElement).disabled).toBe(true);
    expect((screen.getByTestId('security-webhook-url') as HTMLInputElement).disabled).toBe(true);
    expect(screen.queryByTestId('security-notifications-save-btn')).toBeNull();
  });

  it('shows provider security event checkboxes in add-provider flow', async () => {
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => {
      expect(screen.getByTestId('add-provider-btn')).toBeInTheDocument();
    });

    await user.click(screen.getByTestId('add-provider-btn'));
    expect(screen.getByTestId('notify-security-waf-blocks')).toBeInTheDocument();
    expect(screen.getByTestId('notify-security-acl-denies')).toBeInTheDocument();
    expect(screen.getByTestId('notify-security-rate-limit-hits')).toBeInTheDocument();
  });

  it('does not render a modal overlay for security settings', async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByTestId('security-notifications-section')).toBeInTheDocument();
    });

    // Security settings are inline on the page, not inside a modal overlay
    expect(document.querySelector('.fixed.inset-0')).toBeNull();
  });

  it('does not show Shoutrrr help text for telegram provider type', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByTestId('add-provider-btn'));

    const typeSelect = screen.getByTestId('provider-type') as HTMLSelectElement;
    await user.selectOptions(typeSelect, 'telegram');

    // Shoutrrr help text and link must not appear
    expect(screen.queryByText(/shoutrrr/i)).toBeNull();
    expect(document.querySelector('a[href*="containrrr.dev"]')).toBeNull();
  });
});
