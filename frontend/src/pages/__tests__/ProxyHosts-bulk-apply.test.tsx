import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import ProxyHosts from '../ProxyHosts';
import * as proxyHostsApi from '../../api/proxyHosts';
import * as certificatesApi from '../../api/certificates';
import type { Certificate } from '../../api/certificates'
import type { ProxyHost } from '../../api/proxyHosts'
import * as accessListsApi from '../../api/accessLists';
import type { AccessList } from '../../api/accessLists'
import * as settingsApi from '../../api/settings';
import { createMockProxyHost } from '../../testUtils/createMockProxyHost';

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

    await waitFor(() => expect(screen.getByText('Test Host 1')).toBeTruthy());

    // Select first host using select-all checkbox
    const selectAll = screen.getAllByRole('checkbox')[0];
    await userEvent.click(selectAll);

    // Bulk Apply button should appear
    await waitFor(() => expect(screen.getByText('Bulk Apply')).toBeTruthy());

    // Open modal
    await userEvent.click(screen.getByText('Bulk Apply'));
    await waitFor(() => expect(screen.getByText('Bulk Apply Settings')).toBeTruthy());
  });

  it('applies selected settings to all selected hosts by calling updateProxyHost merged payload', async () => {
    const updateMock = vi.mocked(proxyHostsApi.updateProxyHost);
    updateMock.mockResolvedValue(mockProxyHosts[0] as ProxyHost);

    renderWithProviders(<ProxyHosts />);
    await waitFor(() => expect(screen.getByText('Test Host 1')).toBeTruthy());

    // Select hosts
    const selectAll = screen.getAllByRole('checkbox')[0];
    await userEvent.click(selectAll);
    await waitFor(() => expect(screen.getByText('Bulk Apply')).toBeTruthy());

    // Open Bulk Apply modal
    await userEvent.click(screen.getByText('Bulk Apply'));
    await waitFor(() => expect(screen.getByText('Bulk Apply Settings')).toBeTruthy());

    // Enable first setting checkbox (Force SSL)
      // Enable first setting checkbox (Force SSL) - locate by text then find the checkbox inside its container
      const forceLabel = screen.getByText(/Force SSL/i) as HTMLElement;
      let forceContainer: HTMLElement | null = forceLabel;
      while (forceContainer && !forceContainer.querySelector('input[type="checkbox"]')) {
        forceContainer = forceContainer.parentElement
      }
      const forceCheckbox = forceContainer ? (forceContainer.querySelector('input[type="checkbox"]') as HTMLElement | null) : null;
      if (forceCheckbox) await userEvent.click(forceCheckbox as HTMLElement);

    // Click Apply (scope to modal to avoid matching header 'Bulk Apply' button)
    const modalRoot = screen.getByText('Bulk Apply Settings').closest('div');
    const { within } = await import('@testing-library/react');
    const applyButton = modalRoot ? within(modalRoot).getByRole('button', { name: /^Apply$/i }) : screen.getByRole('button', { name: /^Apply$/i });
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
    await waitFor(() => expect(screen.getByText('Test Host 1')).toBeTruthy());
    const selectAll = screen.getAllByRole('checkbox')[0];
    await userEvent.click(selectAll);
    await waitFor(() => expect(screen.getByText('Bulk Apply')).toBeTruthy());
    await userEvent.click(screen.getByText('Bulk Apply'));
    await waitFor(() => expect(screen.getByText('Bulk Apply Settings')).toBeTruthy());

    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    await waitFor(() => expect(screen.queryByText('Bulk Apply Settings')).toBeNull());
  });
});
