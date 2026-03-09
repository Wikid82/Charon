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
  };
});

vi.mock('../../utils/toast', () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

describe('Security Notification Settings on Notifications page', () => {
  let queryClient: ReturnType<typeof createTestQueryClient>;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();
    vi.mocked(notificationsApi.getProviders).mockResolvedValue([]);
    vi.mocked(notificationsApi.getTemplates).mockResolvedValue([]);
    vi.mocked(notificationsApi.getExternalTemplates).mockResolvedValue([]);
  });

  const renderPage = () =>
    render(
      <QueryClientProvider client={queryClient}>
        <MemoryRouter>
          <Notifications />
        </MemoryRouter>
      </QueryClientProvider>
    );

  it('does not render a standalone security notifications section', async () => {
    renderPage();
    await screen.findByTestId('add-provider-btn');
    expect(screen.queryByTestId('security-notifications-section')).toBeNull();
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

    await screen.findByTestId('add-provider-btn');

    // Security settings are inline on the page, not inside a modal overlay
    expect(document.querySelector('.fixed.inset-0')).toBeNull();
  });

  it('defaults to Discord webhook flow while exposing supported provider modes', async () => {
    const user = userEvent.setup();
    renderPage();

    await user.click(await screen.findByTestId('add-provider-btn'));

    const typeSelect = screen.getByTestId('provider-type') as HTMLSelectElement;
    expect(Array.from(typeSelect.options).map((option) => option.value)).toEqual(['discord', 'gotify', 'webhook', 'email']);
    expect(typeSelect.value).toBe('discord');

    const webhookInput = screen.getByTestId('provider-url') as HTMLInputElement;
    expect(webhookInput.placeholder).toContain('discord.com/api/webhooks');
    expect(screen.queryByRole('link')).toBeNull();
  });
});
