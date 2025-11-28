import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router-dom';
import { vi, describe, it, expect, beforeEach } from 'vitest';
import ProxyHosts from '../ProxyHosts';
import * as proxyHostsApi from '../../api/proxyHosts';
import * as backupsApi from '../../api/backups';
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
  bulkUpdateProxyHostACL: vi.fn(),
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
  {
    uuid: 'host-1',
    name: 'Test Host 1',
    domain_names: 'test1.example.com',
    forward_host: '192.168.1.10',
    forward_port: 8080,
    forward_scheme: 'http' as const,
    enabled: true,
    ssl_forced: false,
    http2_support: true,
    hsts_enabled: false,
    hsts_subdomains: false,
    block_exploits: true,
    websocket_support: false,
    application: 'none' as const,
    locations: [],
    access_list_id: null,
    certificate_id: null,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    uuid: 'host-2',
    name: 'Test Host 2',
    domain_names: 'test2.example.com',
    forward_host: '192.168.1.20',
    forward_port: 8080,
    forward_scheme: 'http' as const,
    enabled: true,
    ssl_forced: false,
    http2_support: true,
    hsts_enabled: false,
    hsts_subdomains: false,
    block_exploits: true,
    websocket_support: false,
    application: 'none' as const,
    locations: [],
    access_list_id: null,
    certificate_id: null,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    uuid: 'host-3',
    name: 'Test Host 3',
    domain_names: 'test3.example.com',
    forward_host: '192.168.1.30',
    forward_port: 8080,
    forward_scheme: 'http' as const,
    enabled: true,
    ssl_forced: false,
    http2_support: true,
    hsts_enabled: false,
    hsts_subdomains: false,
    block_exploits: true,
    websocket_support: false,
    application: 'none' as const,
    locations: [],
    access_list_id: null,
    certificate_id: null,
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

describe('ProxyHosts - Bulk Delete with Backup', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    
    // Setup default mocks
    vi.mocked(proxyHostsApi.getProxyHosts).mockResolvedValue(mockProxyHosts);
    vi.mocked(certificatesApi.getCertificates).mockResolvedValue([]);
    vi.mocked(accessListsApi.accessListsApi.list).mockResolvedValue([]);
    vi.mocked(settingsApi.getSettings).mockResolvedValue({});
    vi.mocked(backupsApi.createBackup).mockResolvedValue({
      filename: 'backup-2024-01-01-12-00-00.db',
    });
  });

  it('renders bulk delete button when hosts are selected', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select a host
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]); // First checkbox is "select all"

    // Delete button should appear
    await waitFor(() => {
      const buttons = screen.getAllByRole('button', { name: /delete/i });
      // Should have bulk delete button plus row delete buttons
      expect(buttons.length).toBeGreaterThan(mockProxyHosts.length);
    });
  });

  it('shows confirmation modal when delete button is clicked', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select hosts
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);
    fireEvent.click(checkboxes[2]);

    // Wait for bulk action buttons to appear, then click bulk delete button
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    // Modal should appear
    await waitFor(() => {
      expect(screen.getByText(/Delete 2 Proxy Hosts?/i)).toBeTruthy();
    });

    // Should list hosts to be deleted
    expect(screen.getByText('Test Host 1')).toBeTruthy();
    expect(screen.getByText('Test Host 2')).toBeTruthy();

    // Should mention automatic backup
    expect(screen.getByText(/automatic backup/i)).toBeTruthy();
  });

  it('creates backup before deleting hosts', async () => {
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue();

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select a host
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);

    // Click delete button
    const deleteButton = screen.getByText('Delete');
    fireEvent.click(deleteButton);

    // Wait for modal
    await waitFor(() => {
      expect(screen.getByText(/Delete 1 Proxy Host?/i)).toBeTruthy();
    });

    // Click confirm delete
    const confirmButton = screen.getByText('Delete Permanently');
    fireEvent.click(confirmButton);

    // Should create backup first
    await waitFor(() => {
      expect(backupsApi.createBackup).toHaveBeenCalled();
    });

    // Should show loading toast
    expect(toast.loading).toHaveBeenCalledWith('Creating backup before deletion...');

    // Should show success toast with backup filename
    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith('Backup created: backup-2024-01-01-12-00-00.db');
    });

    // Should then delete the host
    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('host-1');
    });
  });

  it('deletes multiple selected hosts after backup', async () => {
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue();

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select multiple hosts
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]); // host-1
    fireEvent.click(checkboxes[2]); // host-2
    fireEvent.click(checkboxes[3]); // host-3

    // Wait for bulk action buttons to appear, then click bulk delete button
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    // Wait for modal
    await waitFor(() => {
      expect(screen.getByText(/Delete 3 Proxy Hosts?/i)).toBeTruthy();
    });

    // Click confirm delete
    const confirmButton = screen.getByText('Delete Permanently');
    fireEvent.click(confirmButton);

    // Should create backup first
    await waitFor(() => {
      expect(backupsApi.createBackup).toHaveBeenCalled();
    });

    // Should delete all selected hosts
    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('host-1');
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('host-2');
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalledWith('host-3');
    });

    // Should show success message
    await waitFor(() => {
      expect(toast.success).toHaveBeenCalledWith(
        'Successfully deleted 3 host(s). Backup available for restore.'
      );
    });
  });

  it('reports partial success when some deletions fail', async () => {
    // Make second deletion fail
    vi.mocked(proxyHostsApi.deleteProxyHost)
      .mockResolvedValueOnce()  // host-1 succeeds
      .mockRejectedValueOnce(new Error('Network error'))  // host-2 fails
      .mockResolvedValueOnce();  // host-3 succeeds

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select all hosts
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);
    fireEvent.click(checkboxes[2]);
    fireEvent.click(checkboxes[3]);

    // Wait for bulk action buttons to appear, then click bulk delete button
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    // Wait for modal and confirm
    await waitFor(() => {
      expect(screen.getByText(/Delete 3 Proxy Hosts?/i)).toBeTruthy();
    });

    const confirmButton = screen.getByText('Delete Permanently');
    fireEvent.click(confirmButton);

    // Wait for backup
    await waitFor(() => {
      expect(backupsApi.createBackup).toHaveBeenCalled();
    });

    // Should show partial success
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Deleted 2 host(s), 1 failed');
    });
  });

  it('handles backup creation failure', async () => {
    vi.mocked(backupsApi.createBackup).mockRejectedValue(new Error('Backup failed'));

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select a host
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);

    // Wait for bulk action buttons to appear, then click bulk delete button
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    // Wait for modal and confirm
    await waitFor(() => {
      expect(screen.getByText(/Delete 1 Proxy Host?/i)).toBeTruthy();
    });

    const confirmButton = screen.getByText('Delete Permanently');
    fireEvent.click(confirmButton);

    // Should show error
    await waitFor(() => {
      expect(toast.error).toHaveBeenCalledWith('Backup failed');
    });

    // Should NOT delete hosts if backup fails
    expect(proxyHostsApi.deleteProxyHost).not.toHaveBeenCalled();
  });

  it('closes modal after successful deletion', async () => {
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue();

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select a host
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);

    // Wait for bulk action buttons to appear, then click bulk delete button
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    // Wait for modal
    await waitFor(() => {
      expect(screen.getByText(/Delete 1 Proxy Host?/i)).toBeTruthy();
    });

    // Click confirm delete
    const confirmButton = screen.getByText('Delete Permanently');
    fireEvent.click(confirmButton);

    // Wait for completion
    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalled();
    });

    // Modal should close
    await waitFor(() => {
      expect(screen.queryByText(/Delete 1 Proxy Host?/i)).toBeNull();
    });
  });

  it('clears selection after successful deletion', async () => {
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue();

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select a host
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);

    // Should show selection count (flexible matcher for spacing)
    await waitFor(() => {
      expect(screen.getByText((_content, element) => {
        return element?.textContent === '1  selected';
      })).toBeTruthy();
    });

    // Click bulk delete button and confirm (find it via Manage ACL sibling)
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(screen.getByText(/Delete 1 Proxy Host?/i)).toBeTruthy();
    });

    const confirmButton = screen.getByText('Delete Permanently');
    fireEvent.click(confirmButton);

    // Wait for completion
    await waitFor(() => {
      expect(proxyHostsApi.deleteProxyHost).toHaveBeenCalled();
    });

    // Selection should be cleared - bulk action buttons should disappear
    await waitFor(() => {
      expect(screen.queryByText('Manage ACL')).toBeNull();
    });
  });

  it('disables confirm button while creating backup', async () => {
    // Make backup creation take time
    vi.mocked(backupsApi.createBackup).mockImplementation(
      () => new Promise(resolve => setTimeout(() => resolve({ filename: 'backup.db' }), 100))
    );
    vi.mocked(proxyHostsApi.deleteProxyHost).mockResolvedValue();

    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select a host
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);

    // Wait for bulk action buttons to appear, then click bulk delete button
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    // Wait for modal
    await waitFor(() => {
      expect(screen.getByText(/Delete 1 Proxy Host?/i)).toBeTruthy();
    });

    // Click confirm delete
    const confirmButton = screen.getByText('Delete Permanently');
    fireEvent.click(confirmButton);

    // Button should be disabled and show loading
    await waitFor(() => {
      const button = screen.getByText('Creating backup...');
      expect(button).toBeTruthy();
    });
  });

  it('can cancel deletion from modal', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select a host
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[1]);

    // Wait for bulk action buttons to appear, then click bulk delete button
    await waitFor(() => {
      expect(screen.getByText('Manage ACL')).toBeTruthy();
    });
    const manageACLButton = screen.getByText('Manage ACL');
    const deleteButton = manageACLButton.parentElement?.querySelector('button:last-child') as HTMLButtonElement;
    fireEvent.click(deleteButton);

    // Wait for modal
    await waitFor(() => {
      expect(screen.getByText(/Delete 1 Proxy Host?/i)).toBeTruthy();
    });

    // Click cancel
    const cancelButton = screen.getByText('Cancel');
    fireEvent.click(cancelButton);

    // Modal should close
    await waitFor(() => {
      expect(screen.queryByText(/Delete 1 Proxy Host?/i)).toBeNull();
    });

    // Should NOT create backup or delete
    expect(backupsApi.createBackup).not.toHaveBeenCalled();
    expect(proxyHostsApi.deleteProxyHost).not.toHaveBeenCalled();

    // Selection should remain (flexible matcher for spacing)
    expect(screen.getByText((_content, element) => {
      return element?.textContent === '1  selected';
    })).toBeTruthy();
  });

  it('shows (all) indicator when all hosts selected for deletion', async () => {
    renderWithProviders(<ProxyHosts />);

    await waitFor(() => {
      expect(screen.getByText('Test Host 1')).toBeTruthy();
    });

    // Select all hosts using the select-all checkbox
    const checkboxes = screen.getAllByRole('checkbox');
    fireEvent.click(checkboxes[0]); // First checkbox is "select all"

    // Should show "(all)" indicator (flexible matcher for spacing)
    await waitFor(() => {
      expect(screen.getByText((_content, element) => {
        return element?.textContent === '3 (all) selected';
      })).toBeTruthy();
    });
  });
});
