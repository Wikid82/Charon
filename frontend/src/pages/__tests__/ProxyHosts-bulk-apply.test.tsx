import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import * as accessListsApi from '../../api/accessLists';
import * as certificatesApi from '../../api/certificates';
import * as proxyHostsApi from '../../api/proxyHosts';
import * as settingsApi from '../../api/settings';
import { createMockProxyHost } from '../../testUtils/createMockProxyHost';
import ProxyHosts from '../ProxyHosts';

import type { AccessList } from '../../api/accessLists'
import type { Certificate } from '../../api/certificates'
import type { ProxyHost } from '../../api/proxyHosts'


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
  testProxyHostConnection: vi.fn(),
}));

vi.mock('../../api/certificates', () => ({ getCertificates: vi.fn() }));
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }));
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }));
vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}));
vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({ data: [], isLoading: false, error: null })),
  useCreateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useUpdateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useDeleteProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}));

const mockProxyHosts = [
  createMockProxyHost({ uuid: 'host-1', name: 'Test Host 1', domain_names: 'test1.example.com', forward_host: '192.168.1.10' }),
  createMockProxyHost({ uuid: 'host-2', name: 'Test Host 2', domain_names: 'test2.example.com', forward_host: '192.168.1.20' }),
];

const createQueryClient = () => new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 }, mutations: { retry: false } } });

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>{ui}</MemoryRouter>
    </QueryClientProvider>
  );
};

describe('ProxyHosts - Bulk Apply Settings', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(mockProxyHosts as ProxyHost[]);
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([] as Certificate[]);
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([] as AccessList[]);
    vi.mocked(settingsApi.getSettings).mockResolvedValue({} as Record<string, string>);
  });

  it('shows Bulk Apply button when hosts selected and opens modal', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select first host using select-all checkbox
    const selectAll = screen.getAllByRole('checkbox')[0];
    await userEvent.click(selectAll);

    // Bulk Apply button should appear
    expect(await screen.findByText('Bulk Apply')).toBeTruthy();

    // Open modal
    await userEvent.click(screen.getByText('Bulk Apply'));
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy();
  });

  it('applies selected settings to all selected hosts by calling updateProxyHost merged payload', async () => {
    const updateMock = vi.mocked(proxyHostsApi.updateProxyHost);
    updateMock.mockResolvedValue(mockProxyHosts[0] as ProxyHost);

    renderWithProviders(<ProxyHosts />);
    expect(await screen.findByText('Test Host 1')).toBeTruthy();

    // Select hosts
    const selectAll = screen.getByLabelText('Select all rows');
    await userEvent.click(selectAll);
    expect(await screen.findByText('Bulk Apply')).toBeTruthy();

    // Open Bulk Apply modal
    await userEvent.click(screen.getByText('Bulk Apply'));
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy();

    // Enable first setting checkbox (Force SSL) - find the row by text and then get the Radix Checkbox (role="checkbox")
    const forceLabel = screen.getByText(/Force SSL/i) as HTMLElement;
    const forceRow = forceLabel.closest('.p-3') as HTMLElement;
    const { within } = await import('@testing-library/react');
    // The Radix Checkbox has role="checkbox"
    const forceCheckbox = within(forceRow).getAllByRole('checkbox')[0];
    await userEvent.click(forceCheckbox);

    // Click Apply (find the dialog and get the button from the footer)
    const dialog = screen.getByRole('dialog');
    const applyButton = within(dialog).getByRole('button', { name: /^Apply$/i });
    await userEvent.click(applyButton);

    // Should call updateProxyHost for each selected host with merged payload containing ssl_forced
    await waitFor(() => {
      expect(updateMock).toHaveBeenCalled();
      const calls = updateMock.mock.calls;
      expect(calls.length).toBe(2);
      expect(calls[0][1]).toHaveProperty('ssl_forced');
      expect(calls[1][1]).toHaveProperty('ssl_forced');
    });
  });

  it('cancels bulk apply modal when Cancel clicked', async () => {
    renderWithProviders(<ProxyHosts />);
    expect(await screen.findByText('Test Host 1')).toBeTruthy();
    const selectAll = screen.getAllByRole('checkbox')[0];
    await userEvent.click(selectAll);
    expect(await screen.findByText('Bulk Apply')).toBeTruthy();
    await userEvent.click(screen.getByText('Bulk Apply'));
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy();

    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByText('Bulk Apply Settings')).toBeNull());
  });
});
