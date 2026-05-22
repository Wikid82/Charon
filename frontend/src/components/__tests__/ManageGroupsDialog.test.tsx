import { QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import { createTestQueryClient } from '../../test/createTestQueryClient';
import { ManageGroupsDialog } from '../ManageGroupsDialog';

const mockMutateAsync = vi.fn();

vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({
    data: [],
    isLoading: false,
    error: null,
  })),
  useDeleteProxyGroup: vi.fn(() => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  })),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'proxyGroups.manageGroups': 'Manage Groups',
        'proxyGroups.manageGroupsDescription': 'Manage your proxy groups',
        'proxyGroups.noGroups': 'No groups',
        'proxyGroups.createGroup': 'Create Group',
        'proxyGroups.deleteGroup': 'Delete Group',
        'proxyGroups.deleteGroupConfirm': 'Are you sure you want to delete this group?',
        'common.edit': 'Edit',
        'common.delete': 'Delete',
        'common.cancel': 'Cancel',
        // ProxyGroupForm translations (needed when form renders)
        'proxyGroups.editGroup': 'Edit Group',
        'proxyGroups.createGroupDescription': 'Create a new proxy group',
        'proxyGroups.editGroupDescription': 'Edit proxy group',
        'proxyGroups.groupName': 'Group Name',
        'proxyGroups.groupNamePlaceholder': 'Enter group name',
        'proxyGroups.groupDescription': 'Description',
        'proxyGroups.groupDescriptionPlaceholder': 'Enter description',
        'proxyGroups.groupColor': 'Group Color',
        'proxyGroups.customColor': 'Custom color',
        'common.create': 'Create',
        'common.update': 'Update',
        'common.save': 'Save',
      };
      return map[key] ?? key;
    },
  }),
}));

// Also mock useCreateProxyGroup and useUpdateProxyGroup used by ProxyGroupForm
vi.mock('../../hooks/useProxyGroups', () => ({
  useProxyGroups: vi.fn(() => ({
    data: [],
    isLoading: false,
    error: null,
  })),
  useDeleteProxyGroup: vi.fn(() => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  })),
  useCreateProxyGroup: vi.fn(() => ({
    mutateAsync: vi.fn().mockResolvedValue({}),
    isPending: false,
  })),
  useUpdateProxyGroup: vi.fn(() => ({
    mutateAsync: vi.fn().mockResolvedValue({}),
    isPending: false,
  })),
}));

const mockGroups = [
  {
    uuid: 'uuid-1',
    name: 'Production',
    description: 'Prod env',
    color: '#ef4444',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
  {
    uuid: 'uuid-2',
    name: 'Staging',
    description: 'Staging env',
    color: '#6366f1',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  },
];

function renderDialog(props: { open?: boolean; onClose?: () => void } = {}) {
  const qc = createTestQueryClient();
  const mockClose = vi.fn();
  const utils = render(
    <QueryClientProvider client={qc}>
      <ManageGroupsDialog open={props.open ?? true} onClose={props.onClose ?? mockClose} />
    </QueryClientProvider>,
  );
  return { ...utils, mockClose };
}

import { useProxyGroups } from '../../hooks/useProxyGroups';

describe('ManageGroupsDialog', () => {
  const user = userEvent.setup();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useProxyGroups).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useProxyGroups>);
  });

  it('renders with no groups and shows empty state message', () => {
    renderDialog();
    expect(screen.getByText('Manage Groups')).toBeInTheDocument();
    expect(screen.getByText('No groups')).toBeInTheDocument();
  });

  it('renders group list when groups are present', () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useProxyGroups>);

    renderDialog();

    expect(screen.getByText('Production')).toBeInTheDocument();
    expect(screen.getByText('Staging')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /edit production/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /delete production/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /edit staging/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /delete staging/i })).toBeInTheDocument();
  });

  it('opens ProxyGroupForm in edit mode when Edit button is clicked', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useProxyGroups>);

    renderDialog();

    await user.click(screen.getByRole('button', { name: /edit production/i }));

    await waitFor(() => {
      expect(screen.getByText('Edit Group')).toBeInTheDocument();
    });
    expect(screen.getByLabelText(/group name/i)).toHaveValue('Production');
  });

  it('opens ProxyGroupForm in create mode when Create Group button is clicked', async () => {
    renderDialog();

    await user.click(screen.getByRole('button', { name: /create group/i }));

    await waitFor(() => {
      expect(screen.getByText('Manage your proxy groups')).toBeInTheDocument();
    });

    // ProxyGroupForm opens — check for Group Name input (create mode)
    await waitFor(() => {
      expect(screen.getByLabelText(/group name/i)).toBeInTheDocument();
    });
    expect(screen.getByLabelText(/group name/i)).toHaveValue('');
  });

  it('opens delete confirmation dialog when Delete button is clicked', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useProxyGroups>);

    renderDialog();

    await user.click(screen.getByRole('button', { name: /delete production/i }));

    await waitFor(() => {
      expect(screen.getByText('Delete Group')).toBeInTheDocument();
    });
    expect(screen.getByText('Are you sure you want to delete this group?')).toBeInTheDocument();
  });

  it('closes delete confirmation dialog when Cancel is clicked', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useProxyGroups>);

    renderDialog();

    await user.click(screen.getByRole('button', { name: /delete production/i }));

    await waitFor(() => {
      expect(screen.getByText('Delete Group')).toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: /cancel/i }));

    await waitFor(() => {
      expect(screen.queryByText('Are you sure you want to delete this group?')).not.toBeInTheDocument();
    });
  });

  it('calls deleteGroup.mutateAsync with group UUID when confirm delete is clicked', async () => {
    vi.mocked(useProxyGroups).mockReturnValue({
      data: mockGroups,
      isLoading: false,
      error: null,
    } as unknown as ReturnType<typeof useProxyGroups>);
    mockMutateAsync.mockResolvedValueOnce(undefined);

    renderDialog();

    await user.click(screen.getByRole('button', { name: /delete production/i }));

    await waitFor(() => {
      expect(screen.getByText('Delete Group')).toBeInTheDocument();
    });

    // There are two "Delete" buttons now (the row button + the confirm button).
    // The confirm button is inside the confirmation dialog footer.
    const deleteButtons = screen.getAllByRole('button', { name: /^delete$/i });
    // The last one is the confirm button in the dialog footer
    await user.click(deleteButtons[deleteButtons.length - 1]);

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith('uuid-1');
    });

    await waitFor(() => {
      expect(screen.queryByText('Are you sure you want to delete this group?')).not.toBeInTheDocument();
    });
  });
});
