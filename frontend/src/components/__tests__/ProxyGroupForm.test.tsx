import { QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { vi, describe, it, expect, beforeEach } from 'vitest';

import { createTestQueryClient } from '../../test/createTestQueryClient';
import { ProxyGroupForm } from '../ProxyGroupForm';

const mockMutateAsync = vi.fn();

vi.mock('../../hooks/useProxyGroups', () => ({
  useCreateProxyGroup: vi.fn(() => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  })),
  useUpdateProxyGroup: vi.fn(() => ({
    mutateAsync: mockMutateAsync,
    isPending: false,
  })),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: Record<string, unknown>) => {
      const map: Record<string, string> = {
        'proxyGroups.createGroup': 'Create Group',
        'proxyGroups.editGroup': 'Edit Group',
        'proxyGroups.createGroupDescription': 'Create a new proxy group',
        'proxyGroups.editGroupDescription': 'Edit proxy group',
        'proxyGroups.groupName': 'Group Name',
        'proxyGroups.groupNamePlaceholder': 'Enter group name',
        'proxyGroups.groupDescription': 'Description',
        'proxyGroups.groupDescriptionPlaceholder': 'Enter description',
        'proxyGroups.groupColor': 'Group Color',
        'proxyGroups.customColor': 'Custom color',
        'common.cancel': 'Cancel',
        'common.create': 'Create',
        'common.update': 'Update',
      };
      if (key === 'proxyGroups.colorPreset' && options?.color) {
        return `Color ${options.color}`;
      }
      return map[key] ?? key;
    },
  }),
}));

function renderForm(props: Partial<React.ComponentProps<typeof ProxyGroupForm>> = {}) {
  const qc = createTestQueryClient();
  const mockClose = vi.fn();
  const utils = render(
    <QueryClientProvider client={qc}>
      <ProxyGroupForm open={true} onClose={mockClose} {...props} />
    </QueryClientProvider>,
  );
  return { ...utils, mockClose };
}

describe('ProxyGroupForm', () => {
  const user = userEvent.setup();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders create form when no group prop', () => {
    renderForm();
    expect(screen.getByText('Create Group')).toBeInTheDocument();
    expect(screen.getByLabelText(/group name/i)).toBeInTheDocument();
    expect(screen.getByLabelText(/description/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /create/i })).toBeInTheDocument();
  });

  it('renders edit form with pre-filled values when group prop is provided', () => {
    const group = {
      uuid: 'test-uuid',
      name: 'Production',
      description: 'Prod env',
      color: '#ef4444',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    };
    renderForm({ group });
    expect(screen.getByText('Edit Group')).toBeInTheDocument();
    expect(screen.getByLabelText(/group name/i)).toHaveValue('Production');
    expect(screen.getByLabelText(/description/i)).toHaveValue('Prod env');
    expect(screen.getByRole('button', { name: /update/i })).toBeInTheDocument();
  });

  it('disables submit when name is empty', () => {
    renderForm();
    const submitBtn = screen.getByRole('button', { name: /create/i });
    expect(submitBtn).toBeDisabled();
  });

  it('enables submit when name is filled', async () => {
    renderForm();
    await user.type(screen.getByLabelText(/group name/i), 'My Group');
    const submitBtn = screen.getByRole('button', { name: /create/i });
    expect(submitBtn).toBeEnabled();
  });

  it('calls create mutation on submit in create mode', async () => {
    mockMutateAsync.mockResolvedValueOnce({});
    const { mockClose } = renderForm();

    await user.type(screen.getByLabelText(/group name/i), 'New Group');
    await user.click(screen.getByRole('button', { name: /create/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ name: 'New Group' }),
      );
    });
    await waitFor(() => expect(mockClose).toHaveBeenCalled());
  });

  it('calls update mutation on submit in edit mode', async () => {
    mockMutateAsync.mockResolvedValueOnce({});
    const group = {
      uuid: 'edit-uuid',
      name: 'Old Name',
      description: '',
      color: '#6366f1',
      created_at: '2024-01-01T00:00:00Z',
      updated_at: '2024-01-01T00:00:00Z',
    };
    const { mockClose } = renderForm({ group });

    const nameInput = screen.getByLabelText(/group name/i);
    await user.clear(nameInput);
    await user.type(nameInput, 'New Name');
    await user.click(screen.getByRole('button', { name: /update/i }));

    await waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({ uuid: 'edit-uuid', data: expect.objectContaining({ name: 'New Name' }) }),
      );
    });
    await waitFor(() => expect(mockClose).toHaveBeenCalled());
  });

  it('closes when cancel is clicked', async () => {
    const { mockClose } = renderForm();
    await user.click(screen.getByRole('button', { name: /cancel/i }));
    expect(mockClose).toHaveBeenCalled();
  });

  it('closes dialog via Escape key (onOpenChange)', async () => {
    const { mockClose } = renderForm();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(mockClose).toHaveBeenCalled());
  });

  it('updates description when typing in description field', async () => {
    renderForm();
    const descInput = screen.getByLabelText(/description/i);
    await user.type(descInput, 'My description');
    expect(descInput).toHaveValue('My description');
  });

  it('changes color when clicking a non-default color preset', async () => {
    renderForm();
    const redPreset = screen.getByRole('button', { name: /color #ef4444/i });
    await user.click(redPreset);
    expect(redPreset).toHaveAttribute('aria-pressed', 'true');
  });

  it('changes color via custom hex color input', async () => {
    renderForm();
    const colorInput = screen.getByLabelText('Custom color');
    fireEvent.change(colorInput, { target: { value: '#ff0000' } });
    expect(colorInput).toHaveValue('#ff0000');
  });
});
