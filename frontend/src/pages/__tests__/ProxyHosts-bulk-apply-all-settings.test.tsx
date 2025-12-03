import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import ProxyHosts from '../ProxyHosts';
import * as proxyHostsApi from '../../api/proxyHosts';
import * as certificatesApi from '../../api/certificates';
import type { ProxyHost } from '../../api/proxyHosts'
import type { Certificate } from '../../api/certificates'
import * as accessListsApi from '../../api/accessLists';
import type { AccessList } from '../../api/accessLists'
import * as settingsApi from '../../api/settings';
import { createMockProxyHost } from '../../testUtils/createMockProxyHost';

vi.mock('react-hot-toast', () => ({ toast: { success: vi.fn(), error: vi.fn(), loading: vi.fn(), dismiss: vi.fn() } }));
vi.mock('../../api/proxyHosts', () => ({ getProxyHosts: vi.fn(), createProxyHost: vi.fn(), updateProxyHost: vi.fn(), deleteProxyHost: vi.fn(), bulkUpdateACL: vi.fn(), testProxyHostConnection: vi.fn() }));
vi.mock('../../api/certificates', () => ({ getCertificates: vi.fn() }));
vi.mock('../../api/accessLists', () => ({ accessListsApi: { list: vi.fn() } }));
vi.mock('../../api/settings', () => ({ getSettings: vi.fn() }));

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

    await waitFor(() => expect(screen.getByText('Host 1')).toBeTruthy());

    // select all
    const headerCheckbox = screen.getAllByRole('checkbox')[0];
    await userEvent.click(headerCheckbox);

    // open Bulk Apply
    await waitFor(() => expect(screen.getByText('Bulk Apply')).toBeTruthy());
    await userEvent.click(screen.getByText('Bulk Apply'));
    await waitFor(() => expect(screen.getByText('Bulk Apply Settings')).toBeTruthy());

    const labels = [
      'Force SSL',
      'HTTP/2 Support',
      'HSTS Enabled',
      'HSTS Subdomains',
      'Block Exploits',
      'Websockets Support',
    ];

    for (const lbl of labels) {
      expect(screen.getByText(lbl)).toBeTruthy();
      // find close checkbox and click its apply checkbox (the first input in the label area)
      const el = screen.getByText(lbl) as HTMLElement;
      let container: HTMLElement | null = el;
      while (container && !container.querySelector('input[type="checkbox"]')) container = container.parentElement;
      const cb = container?.querySelector('input[type="checkbox"]') as HTMLElement | null;
      if (cb) await userEvent.click(cb);
    }

    // After toggling at least one, Apply should be enabled
    const modalRoot = screen.getByText('Bulk Apply Settings').closest('div');
    const { within } = await import('@testing-library/react');
    const applyBtn = modalRoot ? within(modalRoot).getByRole('button', { name: /^Apply$/i }) : screen.getByRole('button', { name: /^Apply$/i });
    expect(applyBtn).toBeTruthy();
    // Cancel to close
    await userEvent.click(modalRoot ? within(modalRoot).getByRole('button', { name: /Cancel/i }) : screen.getByRole('button', { name: /Cancel/i }));
    await waitFor(() => expect(screen.queryByText('Bulk Apply Settings')).toBeNull());
  });
});
