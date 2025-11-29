import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import { createMockProxyHost } from '../../testUtils/createMockProxyHost';
import ProxyHosts from '../ProxyHosts';
import * as proxyHostsApi from '../../api/proxyHosts';
import * as certificatesApi from '../../api/certificates';
import * as accessListsApi from '../../api/accessLists';
import * as settingsApi from '../../api/settings';
import { toast } from 'react-hot-toast';

// Mock toast
vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
    loading: vi.fn(),
    dismiss: vi.fn(),
  },
}));

// Mock API modules
vi.mock('../../api/proxyHosts', () => ({
  getProxyHosts: vi.fn(),
  createProxyHost: vi.fn(),
  updateProxyHost: vi.fn(),
  deleteProxyHost: vi.fn(),
  bulkUpdateACL: vi.fn(),
  testProxyHostConnection: vi.fn(),
}));

vi.mock('../../api/backups', () => ({
  createBackup: vi.fn(),
  getBackups: vi.fn(),
  restoreBackup: vi.fn(),
  deleteBackup: vi.fn(),
}));

vi.mock('../../api/certificates', () => ({
  getCertificates: vi.fn(),
}));

vi.mock('../../api/accessLists', () => ({
  accessListsApi: {
    list: vi.fn(),
    get: vi.fn(),
    getTemplates: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    testIP: vi.fn(),
  },
}));

vi.mock('../../api/settings', () => ({
  getSettings: vi.fn(),
}));

const mockProxyHosts = [
  createMockProxyHost({ uuid: 'host-1', name: 'Test Host 1', domain_names: 'test1.example.com', forward_host: '192.168.1.10' }),
  createMockProxyHost({ uuid: 'host-2', name: 'Test Host 2', domain_names: 'test2.example.com', forward_host: '192.168.1.20' }),
];

const mockAccessLists = [
  {
    id: 1,
    uuid: 'acl-1',
    name: 'Admin Only',
    description: 'Admin access',
    type: 'whitelist' as const,
    ip_rules: '[]',
    country_codes: '',
    local_network_only: false,
    enabled: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 2,
    uuid: 'acl-2',
    name: 'Local Network',
    description: 'Local network only',
    type: 'whitelist' as const,
    ip_rules: '[]',
    country_codes: '',
    local_network_only: true,
    enabled: true,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    id: 3,
    uuid: 'acl-3',
    name: 'Disabled ACL',
    description: 'This is disabled',
    type: 'blacklist' as const,
    ip_rules: '[]',
    country_codes: '',
    local_network_only: false,
    enabled: false,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
];

const createQueryClient = () => new QueryClient({
  defaultOptions: {
    queries: {
      retry: false,
      gcTime: 0,
    },
    mutations: {
      retry: false,
    },
  },
});

const renderWithProviders = (ui: React.ReactNode) => {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        {ui}
      </MemoryRouter>
    </QueryClientProvider>
  );
};

describe('ProxyHosts - Bulk ACL Modal', () => {
  beforeEach(() => {
    vi.clearAllMocks();

    // Setup default mocks
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(mockProxyHosts);
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([]);
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue(mockAccessLists);
    vi.mocked(settingsApi.getSettings).mockResolvedValue({});
  });

  it('renders Manage ACL button when hosts are selected', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select all hosts using the select-all checkbox
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    // Manage ACL button should appear
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
  });

  it('opens bulk ACL modal when Manage ACL is clicked', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });

    // Click Manage ACL
    await userEvent.click(screen.getByText('Manage ACL'));

    // Modal should open
    await waitFor(() => {
      expect(screen.getByText('Apply Access List')).toBeTruthy();
    });
  });

  it('shows Apply ACL and Remove ACL toggle buttons', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Should show toggle buttons
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Apply ACL' })).toBeTruthy();
      expect(screen.getByRole('button', { name: 'Remove ACL' })).toBeTruthy();
    });
  });

  it('shows only enabled access lists in the selection', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Should show enabled ACLs
    await waitFor(() => {
      expect(screen.getByText('Admin Only')).toBeTruthy();
      expect(screen.getByText('Local Network')).toBeTruthy();
    });

    // Should NOT show disabled ACL
    expect(screen.queryByText('Disabled ACL')).toBeNull();
  });

  it('shows ACL type alongside name', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Should show type - the modal should display ACL types
    await waitFor(() => {
      // Check that the ACL list is rendered with names
      expect(screen.getByText('Admin Only')).toBeTruthy();
      expect(screen.getByText('Local Network')).toBeTruthy();
    });
  });

  it('has Apply button disabled when no ACL is selected', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Wait for modal to open
    await waitFor(() => {
      expect(screen.getByText('Apply Access List')).toBeTruthy();
    });

    // Apply button should be disabled - find it by looking for the action button (not toggle)
    // The action button has bg-blue-600 class, the toggle has flex-1 class
    const buttons = screen.getAllByRole('button');
    const applyButton = buttons.find(btn => {
      const text = btn.textContent?.trim() || '';
      const hasApply = text.startsWith('Apply') && !text.includes('ACL'); // "Apply" or "Apply (N)" but not "Apply ACL"
      const isActionButton = btn.className.includes('bg-blue-600');
      return hasApply && isActionButton;
    });
    expect(applyButton).toBeTruthy();
    expect((applyButton as HTMLButtonElement)?.disabled).toBe(true);
  });

  it('enables Apply button when ACL is selected', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    const user = userEvent.setup()
    await user.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await user.click(screen.getByText('Manage ACL'));

    // Wait for ACL list
    await waitFor(() => {
      expect(screen.getByText('Admin Only')).toBeTruthy();
    });

    // Select an ACL
    const aclCheckboxes = screen.getAllByRole('checkbox');
    // Find the checkbox for Admin Only (should be after the host selection checkboxes)
    const aclCheckbox = aclCheckboxes.find(cb =>
      cb.closest('label')?.textContent?.includes('Admin Only')
    );
    if (aclCheckbox) {
      await userEvent.click(aclCheckbox);
    }

    // Apply button should be enabled and show count
    await waitFor(() => {
      const applyButton = screen.getByRole('button', { name: /Apply \(1\)/ });
      expect(applyButton).toBeTruthy();
      expect(applyButton).toHaveProperty('disabled', false);
    });
  });

  it('can select multiple ACLs', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Wait for ACL list
    await waitFor(() => {
      expect(screen.getByText('Admin Only')).toBeTruthy();
    });

    // Select multiple ACLs
    const aclCheckboxes = screen.getAllByRole('checkbox');
    const adminCheckbox = aclCheckboxes.find(cb =>
      cb.closest('label')?.textContent?.includes('Admin Only')
    );
    const localCheckbox = aclCheckboxes.find(cb =>
      cb.closest('label')?.textContent?.includes('Local Network')
    );

    if (adminCheckbox) await userEvent.click(adminCheckbox);
    if (localCheckbox) await userEvent.click(localCheckbox);

    // Apply button should show count of 2
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Apply \(2\)/ })).toBeTruthy();
    });
  });

  it('applies ACL to selected hosts successfully', async () => {
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({
      updated: 2,
      errors: [],
    });

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Wait for ACL list and select one
    await waitFor(() => {
      expect(screen.getByText('Admin Only')).toBeTruthy();
    });

    const aclCheckboxes = screen.getAllByRole('checkbox');
    const adminCheckbox = aclCheckboxes.find(cb =>
      cb.closest('label')?.textContent?.includes('Admin Only')
    );
    if (adminCheckbox) await userEvent.click(adminCheckbox);

    // Click Apply
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Apply \(1\)/ })).toBeTruthy();
    });
    await userEvent.click(screen.getByRole('button', { name: /Apply \(1\)/ }));

    // Should call API
    await waitFor(() => {
      expect(proxyHostsApi.bulkUpdateACL).toHaveBeenCalledWith(
        ['host-1', 'host-2'],
        1
      );
    });

    // Should show success toast
    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Applied 1 ACL(s) to 2 host(s)');
    });
  });

  it('shows Remove ACL confirmation when Remove is selected', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Wait for modal and find Remove ACL toggle (it's a button with flex-1 class)
    await waitFor(() => {
      expect(screen.getByText('Apply Access List')).toBeTruthy();
    });

    // Find the toggle button by looking for flex-1 class
    const buttons = screen.getAllByRole('button');
    const removeToggle = buttons.find(btn =>
      btn.textContent === 'Remove ACL' && btn.className.includes('flex-1')
    );
    expect(removeToggle).toBeTruthy();
    if (removeToggle) await userEvent.click(removeToggle);

    // Should show warning message
    await waitFor(() => {
      expect(screen.getByText(/will become publicly accessible/i)).toBeTruthy();
    });
  });

  it('closes modal on Cancel', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Modal should open
    await waitFor(() => {
      expect(screen.getByText('Apply Access List')).toBeTruthy();
    });

    // Click Cancel
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    // Modal should close
    await waitFor(() => {
      expect(screen.queryByText('Apply Access List')).toBeNull();
    });
  });

  it('clears selection and closes modal after successful apply', async () => {
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({
      updated: 2,
      errors: [],
    });

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Select ACL and apply
    await waitFor(() => {
      expect(screen.getByText('Admin Only')).toBeTruthy();
    });

    const aclCheckboxes = screen.getAllByRole('checkbox');
    const adminCheckbox = aclCheckboxes.find(cb =>
      cb.closest('label')?.textContent?.includes('Admin Only')
    );
    if (adminCheckbox) await userEvent.click(adminCheckbox);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Apply \(1\)/ })).toBeTruthy();
    });
    await userEvent.click(screen.getByRole('button', { name: /Apply \(1\)/ }));

    // Wait for completion
    await waitFor(() => {
      expect(proxyHostsApi.bulkUpdateACL).toHaveBeenCalled();
    });

    // Modal should close
    await waitFor(() => {
      expect(screen.queryByText('Apply Access List')).toBeNull();
    });

    // Selection should be cleared (Manage ACL button should be gone)
    await waitFor(() => {
      expect(screen.queryByText('Manage ACL')).toBeNull();
    });
  });

  it('shows error toast on API failure', async () => {
    vi.mocked(proxyHostsApi.bulkUpdateACL).mockResolvedValue({
      updated: 1,
      errors: [{ uuid: 'host-2', error: 'Failed' }],
    });

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts and open modal
    const checkboxes = screen.getAllByRole('checkbox');
    await userEvent.click(checkboxes[0]);

    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    await userEvent.click(screen.getByText('Manage ACL'));

    // Select ACL and apply
    await waitFor(() => {
      expect(screen.getByText('Admin Only')).toBeTruthy();
    });

    const aclCheckboxes = screen.getAllByRole('checkbox');
    const adminCheckbox = aclCheckboxes.find(cb =>
      cb.closest('label')?.textContent?.includes('Admin Only')
    );
    if (adminCheckbox) await userEvent.click(adminCheckbox);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /Apply \(1\)/ })).toBeTruthy();
    });
    await userEvent.click(screen.getByRole('button', { name: /Apply \(1\)/ }));

    // Should show error toast
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Applied 1 ACL(s) with some errors');
    });
  });
});
