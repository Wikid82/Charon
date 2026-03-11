import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import * as accessListsApi from '../../api/accessLists';
import * as certificatesApi from '../../api/certificates';
import * as proxyHostsApi from '../../api/proxyHosts';
import * as securityHeadersApi from '../../api/securityHeaders';
import * as settingsApi from '../../api/settings';
import { createMockProxyHost } from '../../testUtils/createMockProxyHost';
import ProxyHosts from '../ProxyHosts';

import type { AccessList } from '../../api/accessLists';
import type { Certificate } from '../../api/certificates';
import type { ProxyHost } from '../../api/proxyHosts';
import type { SecurityHeaderProfile } from '../../api/securityHeaders';


// Mock toast
vi.mock('react-hot-toast', () => ({
  toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() },
}));

vi.mock('../../api/proxyHosts', () => ({
  getProxyHosts: vi.fn(),
  createProxyHost: vi.fn(),
  updateProxyHost: vi.fn(),
  deleteProxyHost: vi.fn(),
  bulkUpdateACL: vi.fn(),
  bulkUpdateSecurityHeaders: vi.fn(),
  testProxyHostConnection: vi.fn(),
}));

vi.mock('../../api/certificates', () => ({ getCertificates: vi.fn() }));
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }));
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }));
vi.mock('../../api/securityHeaders', () => ({
  securityHeadersApi: {
    listProfiles: vi.fn(),
  },
}));

const mockProxyHosts = [
  createMockProxyHost({
    uuid: 'host-1',
    name: 'Test Host 1',
    domain_names: 'test1.example.com',
    forward_host: '192.168.1.10',
  }),
  createMockProxyHost({
    uuid: 'host-2',
    name: 'Test Host 2',
    domain_names: 'test2.example.com',
    forward_host: '192.168.1.20',
  }),
];

const mockSecurityProfiles: SecurityHeaderProfile[] = [
  {
    id: 1,
    uuid: 'profile-1',
    name: 'Strict Security',
    description: 'Maximum security headers',
    security_score: 95,
    is_preset: true,
    preset_type: 'strict',
    hsts_enabled: true,
    hsts_max_age: 31536000,
    hsts_include_subdomains: true,
    hsts_preload: true,
    x_frame_options: 'DENY',
    x_content_type_options: true,
    xss_protection: true,
    referrer_policy: 'no-referrer',
    permissions_policy: '',
    csp_enabled: false,
    csp_directives: '',
    csp_report_only: false,
    csp_report_uri: '',
    cross_origin_opener_policy: 'same-origin',
    cross_origin_resource_policy: 'same-origin',
    cross_origin_embedder_policy: '',
    cache_control_no_store: false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 2,
    uuid: 'profile-2',
    name: 'Moderate Security',
    description: 'Balanced security headers',
    security_score: 75,
    is_preset: true,
    preset_type: 'basic',
    hsts_enabled: true,
    hsts_max_age: 31536000,
    hsts_include_subdomains: false,
    hsts_preload: false,
    x_frame_options: 'SAMEORIGIN',
    x_content_type_options: true,
    xss_protection: true,
    referrer_policy: 'strict-origin-when-cross-origin',
    permissions_policy: '',
    csp_enabled: false,
    csp_directives: '',
    csp_report_only: false,
    csp_report_uri: '',
    cross_origin_opener_policy: 'same-origin',
    cross_origin_resource_policy: 'same-origin',
    cross_origin_embedder_policy: '',
    cache_control_no_store: false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
  {
    id: 3,
    uuid: 'profile-3',
    name: 'Custom Profile',
    description: 'My custom headers',
    security_score: 60,
    is_preset: false,
    preset_type: '',
    hsts_enabled: false,
    hsts_max_age: 0,
    hsts_include_subdomains: false,
    hsts_preload: false,
    x_frame_options: 'SAMEORIGIN',
    x_content_type_options: true,
    xss_protection: true,
    referrer_policy: 'same-origin',
    permissions_policy: '',
    csp_enabled: false,
    csp_directives: '',
    csp_report_only: false,
    csp_report_uri: '',
    cross_origin_opener_policy: 'same-origin',
    cross_origin_resource_policy: 'same-origin',
    cross_origin_embedder_policy: '',
    cache_control_no_store: false,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  },
];

const createQueryClient = () =>
  new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } },
  });

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
};

describe('ProxyHosts - Bulk Apply Security Headers', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(mockProxyHosts as ProxyHost[]);
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([] as Certificate[]);
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([] as AccessList[]);
    vi.mocked(settingsApi.getSettings).mockResolvedValue({} as Record<string, string>);
    vi.mocked(securityHeadersApi.securityHeadersApi.listProfiles).mockResolvedValue(
      mockSecurityProfiles
    );
  });

  it('shows security header profile option in bulk apply modal', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);

    // Open Bulk Apply modal
    const bulkApplyButton = screen.getByText('Bulk Apply');
    await userEvent.click(bulkApplyButton);

    // Check for security header profile section
    await waitFor(() => {
      expect(screen.getByText('Security Header Profile')).toBeTruthy();
      expect(
        screen.getByText('Apply a security header profile to all selected hosts')
      ).toBeTruthy();
    });
  });

  it('enables profile selection when checkbox is checked', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy();

    // Find security header checkbox
    const securityHeaderLabel = screen.getByText('Security Header Profile');
    const securityHeaderRow = securityHeaderLabel.closest('.p-3') as HTMLElement;
    const securityHeaderCheckbox = within(securityHeaderRow).getByRole('checkbox');

    // Dropdown should not be visible initially
    expect(screen.queryByRole('combobox')).toBeNull();

    // Click checkbox to enable
    await userEvent.click(securityHeaderCheckbox);

    // Dropdown should now be visible
    await waitFor(() => {
      expect(screen.getByRole('combobox')).toBeTruthy();
    });
  });

  it('lists all available profiles in dropdown grouped correctly', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    // Enable security header option
    const securityHeaderLabel = screen.getByText('Security Header Profile');
    const securityHeaderRow = securityHeaderLabel.closest('.p-3') as HTMLElement;
    const securityHeaderCheckbox = within(securityHeaderRow).getByRole('checkbox');
    await userEvent.click(securityHeaderCheckbox);

    // Check dropdown options
    await waitFor(() => {
      const dropdown = screen.getByRole('combobox') as HTMLSelectElement;
      expect(dropdown).toBeTruthy();

      // Check for "None" option
      const noneOption = within(dropdown).getByText(/None \(Remove Profile\)/i);
      expect(noneOption).toBeTruthy();

      // Check for preset profiles
      expect(within(dropdown).getByText(/Strict Security/)).toBeTruthy();
      expect(within(dropdown).getByText(/Moderate Security/)).toBeTruthy();

      // Check for custom profiles
      expect(within(dropdown).getByText(/Custom Profile/)).toBeTruthy();
    });
  });

  it('applies security header profile to selected hosts using bulk endpoint', async () => {
    const bulkUpdateMock = vi.mocked(proxyHostsApi.bulkUpdateSecurityHeaders);
    bulkUpdateMock.mockResolvedValue({ updated: 2, errors: [] });

    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    // Enable security header option
    const securityHeaderLabel = screen.getByText('Security Header Profile');
    const securityHeaderRow = securityHeaderLabel.closest('.p-3') as HTMLElement;
    const securityHeaderCheckbox = within(securityHeaderRow).getByRole('checkbox');
    await userEvent.click(securityHeaderCheckbox);

    // Select a profile
    expect(await screen.findByRole('combobox')).toBeTruthy();
    const dropdown = screen.getByRole('combobox') as HTMLSelectElement;
    await userEvent.selectOptions(dropdown, '1'); // Select profile ID 1

    // Click Apply
    const dialog = screen.getByRole('dialog');
    const applyButton = within(dialog).getByRole('button', { name: /^Apply$/i });
    await userEvent.click(applyButton);

    // Verify bulk endpoint was called with correct parameters
    await waitFor(() => {
      expect(bulkUpdateMock).toHaveBeenCalledWith(['host-1', 'host-2'], 1);
    });
  });

  it('removes security header profile when "None" selected', async () => {
    const bulkUpdateMock = vi.mocked(proxyHostsApi.bulkUpdateSecurityHeaders);
    bulkUpdateMock.mockResolvedValue({ updated: 2, errors: [] });

    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    // Enable security header option
    const securityHeaderLabel = screen.getByText('Security Header Profile');
    const securityHeaderRow = securityHeaderLabel.closest('.p-3') as HTMLElement;
    const securityHeaderCheckbox = within(securityHeaderRow).getByRole('checkbox');
    await userEvent.click(securityHeaderCheckbox);

    // Select "None" (value 0)
    expect(await screen.findByRole('combobox')).toBeTruthy();
    const dropdown = screen.getByRole('combobox') as HTMLSelectElement;
    await userEvent.selectOptions(dropdown, '0');

    // Verify warning is shown
    await waitFor(() => {
      expect(
        screen.getByText(
          /This will remove the security header profile from all selected hosts/
        )
      ).toBeTruthy();
    });

    // Click Apply
    const dialog = screen.getByRole('dialog');
    const applyButton = within(dialog).getByRole('button', { name: /^Apply$/i });
    await userEvent.click(applyButton);

    // Verify null was sent to API (remove profile)
    await waitFor(() => {
      expect(bulkUpdateMock).toHaveBeenCalledWith(['host-1', 'host-2'], null);
    });
  });

  it('disables Apply button when no options selected', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy();

    // Apply button should be disabled when nothing is selected
    const dialog = screen.getByRole('dialog');
    const applyButton = within(dialog).getByRole('button', { name: /^Apply$/i });
    expect(applyButton).toHaveProperty('disabled', true);
  });

  it('handles partial failure with appropriate toast', async () => {
    const bulkUpdateMock = vi.mocked(proxyHostsApi.bulkUpdateSecurityHeaders);
    bulkUpdateMock.mockResolvedValue({
      updated: 1,
      errors: [{ uuid: 'host-2', error: 'Profile not found' }],
    });

    const toast = await import('react-hot-toast');

    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    // Enable security header option and select a profile
    const securityHeaderLabel = screen.getByText('Security Header Profile');
    const securityHeaderRow = securityHeaderLabel.closest('.p-3') as HTMLElement;
    const securityHeaderCheckbox = within(securityHeaderRow).getByRole('checkbox');
    await userEvent.click(securityHeaderCheckbox);

    expect(await screen.findByRole('combobox')).toBeTruthy();
    const dropdown = screen.getByRole('combobox') as HTMLSelectElement;
    await userEvent.selectOptions(dropdown, '1');

    // Click Apply
    const dialog = screen.getByRole('dialog');
    const applyButton = within(dialog).getByRole('button', { name: /^Apply$/i });
    await userEvent.click(applyButton);

    // Verify error toast was called
    await waitFor(() => {
      expect(toast.toast.error).toHaveBeenCalled();
    });
  });

  it('resets state on modal close', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    // Enable security header option and select a profile
    const securityHeaderLabel = screen.getByText('Security Header Profile');
    const securityHeaderRow = securityHeaderLabel.closest('.p-3') as HTMLElement;
    const securityHeaderCheckbox = within(securityHeaderRow).getByRole('checkbox');
    await userEvent.click(securityHeaderCheckbox);

    expect(await screen.findByRole('combobox')).toBeTruthy();
    const dropdown = screen.getByRole('combobox') as HTMLSelectElement;
    await userEvent.selectOptions(dropdown, '1');

    // Close modal
    const dialog = screen.getByRole('dialog');
    const cancelButton = within(dialog).getByRole('button', { name: /Cancel/i });
    await userEvent.click(cancelButton);

    // Re-open modal
    await waitFor(() => expect(screen.queryByText('Bulk Apply Settings')).toBeNull());
    await userEvent.click(screen.getByText('Bulk Apply'));

    // Security header checkbox should be unchecked (state was reset)
    await waitFor(() => {
      const securityHeaderLabel2 = screen.getByText('Security Header Profile');
      const securityHeaderRow2 = securityHeaderLabel2.closest('.p-3') as HTMLElement;
      const securityHeaderCheckbox2 = within(securityHeaderRow2).getByRole('checkbox');
      expect(securityHeaderCheckbox2).toHaveAttribute('data-state', 'unchecked');
    });
  });

  it('shows profile description when profile is selected', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts and open modal
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    await userEvent.click(screen.getByText('Bulk Apply'));

    // Enable security header option
    const securityHeaderLabel = screen.getByText('Security Header Profile');
    const securityHeaderRow = securityHeaderLabel.closest('.p-3') as HTMLElement;
    const securityHeaderCheckbox = within(securityHeaderRow).getByRole('checkbox');
    await userEvent.click(securityHeaderCheckbox);

    // Select a profile
    expect(await screen.findByRole('combobox')).toBeTruthy();
    const dropdown = screen.getByRole('combobox') as HTMLSelectElement;
    await userEvent.selectOptions(dropdown, '1'); // Strict Security

    // Verify description is shown
    await waitFor(() => {
      expect(screen.getByText('Maximum security headers')).toBeTruthy();
    });
  });
});
