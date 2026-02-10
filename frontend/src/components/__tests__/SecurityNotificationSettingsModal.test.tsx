import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClientProvider } from '@tanstack/react-query';
import { SecurityNotificationSettingsModal } from '../SecurityNotificationSettingsModal';
import { createTestQueryClient } from '../../test/createTestQueryClient';
import * as notificationsApi from '../../api/notifications';

// Mock the API
vi.mock('../../api/notifications', async () => {
  const actual = await vi.importActual('../../api/notifications');
  return {
    ...actual,
    getSecurityNotificationSettings: vi.fn(),
    updateSecurityNotificationSettings: vi.fn(),
  };
});

// Mock toast
vi.mock('../../utils/toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

describe('SecurityNotificationSettingsModal', () => {
  const mockSettings: notificationsApi.SecurityNotificationSettings = {
    enabled: true,
    min_log_level: 'warn',
    notify_waf_blocks: true,
    notify_acl_denials: true,
    notify_rate_limit_hits: false,
    webhook_url: 'https://example.com/webhook',
    email_recipients: 'admin@example.com',
  };

  let queryClient: ReturnType<typeof createTestQueryClient>;

  beforeEach(() => {
    queryClient = createTestQueryClient();
    vi.clearAllMocks();

    vi.mocked(notificationsApi.getSecurityNotificationSettings).mockResolvedValue(mockSettings);
    vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockResolvedValue(mockSettings);
  });

  const renderModal = (isOpen = true, onClose = vi.fn()) => {
    return render(
      <QueryClientProvider client={queryClient}>
        <SecurityNotificationSettingsModal isOpen={isOpen} onClose={onClose} />
      </QueryClientProvider>
    );
  };

  it('does not render when isOpen is false', () => {
    renderModal(false);
    expect(screen.queryByText('Security Notification Settings')).toBeFalsy();
  });

  it('renders the modal when isOpen is true', async () => {
    renderModal();

    await waitFor(() => {
      expect(screen.getByText('Security Notification Settings')).toBeTruthy();
    });
  });

  it('loads and displays existing settings', async () => {
    renderModal();

    await waitFor(() => {
      const enableSwitch = screen.getByLabelText('Enable Notifications') as HTMLInputElement;
      const levelSelect = screen.getByLabelText(/minimum log level/i) as HTMLSelectElement;
      const webhookInput = screen.getByPlaceholderText(/your-webhook-endpoint/i) as HTMLInputElement;

      expect(enableSwitch.checked).toBe(true);
      expect(levelSelect.value).toBe('warn');
      expect(webhookInput.value).toBe('https://example.com/webhook');
    });
  });

  it('closes modal when close button is clicked', async () => {
    const user = userEvent.setup();
    const mockOnClose = vi.fn();
    renderModal(true, mockOnClose);

    await waitFor(() => {
      expect(screen.getByText('Security Notification Settings')).toBeTruthy();
    });

    const closeButton = screen.getByLabelText('Close');
    await user.click(closeButton);

    expect(mockOnClose).toHaveBeenCalled();
  });

  it('closes modal when clicking outside', async () => {
    const user = userEvent.setup();
    const mockOnClose = vi.fn();
    const { container } = renderModal(true, mockOnClose);

    await waitFor(() => {
      expect(screen.getByText('Security Notification Settings')).toBeTruthy();
    });

    // Click on the backdrop
    const backdrop = container.querySelector('.fixed.inset-0');
    if (backdrop) {
      await user.click(backdrop);
      expect(mockOnClose).toHaveBeenCalled();
    }
  });

  it('submits updated settings', async () => {
    const user = userEvent.setup();
    const mockOnClose = vi.fn();
    renderModal(true, mockOnClose);

    await waitFor(() => {
      expect(screen.getByLabelText('Enable Notifications')).toBeTruthy();
    });

    // Change minimum log level
    const levelSelect = screen.getByLabelText(/minimum log level/i);
    await user.selectOptions(levelSelect, 'error');

    // Change webhook URL
    const webhookInput = screen.getByPlaceholderText(/your-webhook-endpoint/i);
    await user.clear(webhookInput);
    await user.type(webhookInput, 'https://new-webhook.com');

    // Submit form
    const saveButton = screen.getByRole('button', { name: /save settings/i });
    await user.click(saveButton);

    await waitFor(() => {
      expect(notificationsApi.updateSecurityNotificationSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          min_log_level: 'error',
          webhook_url: 'https://new-webhook.com',
        })
      );
    });

    // Modal should close on success
    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it('toggles notification enable/disable', async () => {
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByLabelText('Enable Notifications')).toBeChecked();
    });

    const enableSwitch = screen.getByLabelText('Enable Notifications') as HTMLInputElement;

    // Disable notifications
    await user.click(enableSwitch);

    await waitFor(() => {
      expect(enableSwitch.checked).toBe(false);
    });
  });

  it('disables controls when notifications are disabled', async () => {
    vi.mocked(notificationsApi.getSecurityNotificationSettings).mockResolvedValue({
      ...mockSettings,
      enabled: false,
    });

    renderModal();

    // Wait for settings to be loaded and form to render
    await waitFor(() => {
      const enableSwitch = screen.getByLabelText('Enable Notifications') as HTMLInputElement;
      expect(enableSwitch.checked).toBe(false);
    });

    const levelSelect = screen.getByLabelText(/minimum log level/i) as HTMLSelectElement;
    expect(levelSelect.disabled).toBe(true);

    const webhookInput = screen.getByPlaceholderText(/your-webhook-endpoint/i) as HTMLInputElement;
    expect(webhookInput.disabled).toBe(true);
  });

  it('toggles event type filters', async () => {
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByText('WAF Blocks')).toBeTruthy();
    });

    // Find and toggle WAF blocks switch
    const wafSwitch = screen.getByLabelText('WAF Blocks') as HTMLInputElement;
    expect(wafSwitch.checked).toBe(true);

    await user.click(wafSwitch);

    // Submit form
    const saveButton = screen.getByRole('button', { name: /save settings/i });
    await user.click(saveButton);

    await waitFor(() => {
      expect(notificationsApi.updateSecurityNotificationSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          notify_waf_blocks: false,
        })
      );
    });
  });

  it('handles API errors gracefully', async () => {
    const user = userEvent.setup();
    const mockOnClose = vi.fn();

    vi.mocked(notificationsApi.updateSecurityNotificationSettings).mockRejectedValue(
      new Error('API Error')
    );

    renderModal(true, mockOnClose);

    await waitFor(() => {
      expect(screen.getByText('Security Notification Settings')).toBeTruthy();
    });

    // Submit form
    const saveButton = screen.getByRole('button', { name: /save settings/i });
    await user.click(saveButton);

    await waitFor(() => {
      expect(notificationsApi.updateSecurityNotificationSettings).toHaveBeenCalled();
    });

    // Modal should NOT close on error
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it('shows loading state', () => {
    vi.mocked(notificationsApi.getSecurityNotificationSettings).mockReturnValue(
      new Promise(() => {}) // Never resolves
    );

    renderModal();

    expect(screen.getByText('Loading settings...')).toBeTruthy();
  });

  it('handles email recipients input', async () => {
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByPlaceholderText(/admin@example.com/i)).toBeTruthy();
    });

    const emailInput = screen.getByPlaceholderText(/admin@example.com/i);
    await user.clear(emailInput);
    await user.type(emailInput, 'user1@test.com, user2@test.com');

    const saveButton = screen.getByRole('button', { name: /save settings/i });
    await user.click(saveButton);

    await waitFor(() => {
      expect(notificationsApi.updateSecurityNotificationSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          email_recipients: 'user1@test.com, user2@test.com',
        })
      );
    });
  });

  it('prevents modal content clicks from closing modal', async () => {
    const user = userEvent.setup();
    const mockOnClose = vi.fn();
    renderModal(true, mockOnClose);

    await waitFor(() => {
      expect(screen.getByText('Security Notification Settings')).toBeTruthy();
    });

    // Click inside the modal content
    const modalContent = screen.getByText('Security Notification Settings');
    await user.click(modalContent);

    // Modal should not close
    expect(mockOnClose).not.toHaveBeenCalled();
  });
});
