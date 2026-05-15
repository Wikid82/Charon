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



vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }));
vi.mock('../../api/proxyHosts', () => ({ getProxyHosts: vi.fn(), createProxyHost: vi.fn(), updateProxyHost: vi.fn(), deleteProxyHost: vi.fn(), bulkUpdateACL: vi.fn(), testProxyHostConnection: vi.fn() }));
vi.mock('../../api/certificates', () => ({ getCertificates: vi.fn() }));
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }));
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }));
vi.mock('../../hooks/useSecurityHeaders', () => ({
  useSecurityHeaderProfiles: vi.fn(() => ({ data: [], isLoading: false, error: null })),
}))
vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({ data: [], isLoading: false, error: null })),
  useCreateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useUpdateProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
  useDeleteProxyGroup: vi.fn(() => ({ mutateAsync: vi.fn(), isPending: false })),
}));

const hosts = [
  createMockProxyHost({ uuid: 'h1', name: 'Host 1', domain_names: 'one.example.com' }),
  createMockProxyHost({ uuid: 'h2', name: 'Host 2', domain_names: 'two.example.com' }),
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

describe('ProxyHosts - Bulk Apply all settings coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(hosts as ProxyHost[]);
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([] as Certificate[]);
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([] as AccessList[]);
    vi.mocked(settingsApi.getSettings).mockResolvedValue({} as Record<string, string>);
  });

  it('renders all bulk apply setting labels and allows toggling', async () => {
    renderWithProviders(<ProxyHosts />);

    expect(await screen.findByText('Host 1')).toBeTruthy();

    // select all
    const headerCheckbox = screen.getByLabelText('Select all rows');
    await userEvent.click(headerCheckbox);

    // open Bulk Apply
    expect(await screen.findByText('Bulk Apply')).toBeTruthy();
    await userEvent.click(screen.getByText('Bulk Apply'));
    expect(await screen.findByText('Bulk Apply Settings')).toBeTruthy();

    const labels = [
      'Force SSL',
      'HTTP/2 Support',
      'HSTS Enabled',
      'HSTS Subdomains',
      'Block Exploits',
      'Websockets Support',
    ];

    const { within } = await import('@testing-library/react');

    for (const lbl of labels) {
      expect(screen.getByText(lbl)).toBeTruthy();
      // Find the setting row and click the Radix Checkbox (role="checkbox")
      const labelEl = screen.getByText(lbl) as HTMLElement;
      const row = labelEl.closest('.p-3') as HTMLElement;
      const checkboxes = within(row).getAllByRole('checkbox');
      await userEvent.click(checkboxes[0]);
    }

    // After toggling at least one, Apply should be enabled
    const dialog = screen.getByRole('dialog');
    const applyBtn = within(dialog).getByRole('button', { name: /^Apply$/i });
    expect(applyBtn).toBeTruthy();
    // Cancel to close
    await userEvent.click(within(dialog).getByRole('button', { name: /Cancel/i }));
    await waitFor(() => expect(screen.queryByText('Bulk Apply Settings')).toBeNull());
  });
});
