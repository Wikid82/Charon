import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { type OrthrusAgent } from '../../../api/orthrus';
import { OrthrusAgentManager } from '../OrthrusAgentManager';

const mockDelete = vi.fn();
const mockRename = vi.fn();

vi.mock('../../../hooks/useOrthrus', () => ({
  useDeleteAgent: () => ({ mutate: mockDelete, isPending: false }),
  useRenameAgent: () => ({ mutate: mockRename, isPending: false }),
}));

vi.mock('../AgentProviderAssignDialog', () => ({
  AgentProviderAssignDialog: ({
    open,
    onClose,
    agent,
  }: {
    open: boolean;
    onClose: () => void;
    agent: { name: string };
  }) =>
    open ? (
      <div data-testid="assign-dialog" aria-label={`assign-dialog-${agent.name}`}>
        <button onClick={onClose}>CloseAssign</button>
      </div>
    ) : null,
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) =>
      opts?.name ? `${key}:${opts.name}` : key,
  }),
}));

const agentWithProvider = {
  uuid: 'agent-1',
  name: 'Prod Agent',
  status: 'online' as const,
  capabilities: '["proxy"]',
  hecate_tunnel_uuid: 'ts-uuid',
  resolved_address: '100.72.3.4',
  device_id: 'ts-device-1',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

const agentWithoutProvider = {
  uuid: 'agent-2',
  name: 'Dev Agent',
  status: 'offline' as const,
  capabilities: '[]',
  hecate_tunnel_uuid: undefined,
  resolved_address: undefined,
  device_id: undefined,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

const agentWithDeviceIdOnly = {
  uuid: 'agent-3',
  name: 'Device Agent',
  status: 'online' as const,
  capabilities: '[]',
  hecate_tunnel_uuid: 'ts-uuid',
  resolved_address: undefined,
  device_id: 'ts-device-abc',
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

const agentTunnelOnly = {
  uuid: 'agent-4',
  name: 'Tunnel Only Agent',
  status: 'online' as const,
  capabilities: '[]',
  hecate_tunnel_uuid: 'ts-uuid',
  resolved_address: undefined,
  device_id: undefined,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 0,
};

function renderManager(agents: OrthrusAgent[]) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={qc}>
      <OrthrusAgentManager agents={agents} />
    </QueryClientProvider>,
  );
}

describe('OrthrusAgentManager', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders empty-state message when agents array is empty', () => {
    renderManager([]);

    expect(screen.getByText('hecate.agentManager.noAgents')).toBeInTheDocument();
    expect(screen.queryByRole('table')).not.toBeInTheDocument();
  });

  it('renders agent table when agents are provided', () => {
    renderManager([agentWithProvider]);

    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('Prod Agent')).toBeInTheDocument();
  });

  it('shows resolved_address when agent has a provider', () => {
    renderManager([agentWithProvider]);

    expect(screen.getByText('100.72.3.4')).toBeInTheDocument();
  });

  it('shows "no provider assigned" text when agent has no hecate_tunnel_uuid', () => {
    renderManager([agentWithoutProvider]);

    expect(
      screen.getByText('hecate.agentManager.noProviderAssigned'),
    ).toBeInTheDocument();
  });

  it('clicking delete button opens confirm dialog', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.deleteLabel:${agentWithProvider.name}`,
      }),
    );

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument();
    });
  });

  it('clicking delete confirm calls deleteAgent', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.deleteLabel:${agentWithProvider.name}`,
      }),
    );

    await waitFor(() => screen.getByRole('dialog'));

    fireEvent.click(screen.getByText('hecate.agentManager.deleteConfirm'));

    expect(mockDelete).toHaveBeenCalledWith(
      agentWithProvider.uuid,
      expect.objectContaining({ onSettled: expect.any(Function) }),
    );
  });

  it('clicking delete cancel closes dialog without calling deleteAgent', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.deleteLabel:${agentWithProvider.name}`,
      }),
    );

    await waitFor(() => screen.getByRole('dialog'));

    fireEvent.click(screen.getByText('common.cancel'));

    expect(mockDelete).not.toHaveBeenCalled();
  });

  it('clicking assign provider button opens AgentProviderAssignDialog', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.assignProvider:${agentWithProvider.name}`,
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId('assign-dialog')).toBeInTheDocument();
    });
  });

  it('AgentProviderAssignDialog is not rendered initially', () => {
    renderManager([agentWithProvider]);

    expect(screen.queryByTestId('assign-dialog')).not.toBeInTheDocument();
  });

  it('closing assign provider dialog hides it', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.assignProvider:${agentWithProvider.name}`,
      }),
    );

    await waitFor(() => screen.getByTestId('assign-dialog'));

    fireEvent.click(screen.getByText('CloseAssign'));

    await waitFor(() => {
      expect(screen.queryByTestId('assign-dialog')).not.toBeInTheDocument();
    });
  });

  it('clicking rename button shows inline rename input', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.editNameLabel:${agentWithProvider.name}`,
      }),
    );

    await waitFor(() => {
      expect(
        screen.getByRole('textbox', {
          name: `hecate.agentManager.renameInputLabel:${agentWithProvider.name}`,
        }),
      ).toBeInTheDocument();
    });
  });

  it('submitting rename calls renameAgent with new name', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.editNameLabel:${agentWithProvider.name}`,
      }),
    );

    const input = await screen.findByRole('textbox', {
      name: `hecate.agentManager.renameInputLabel:${agentWithProvider.name}`,
    });

    fireEvent.change(input, { target: { value: 'New Name' } });
    fireEvent.click(screen.getByRole('button', { name: 'hecate.agentManager.confirmRename' }));

    expect(mockRename).toHaveBeenCalledWith(
      { uuid: agentWithProvider.uuid, name: 'New Name' },
      expect.anything(),
    );
  });

  it('canceling rename hides the inline rename input', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.editNameLabel:${agentWithProvider.name}`,
      }),
    );

    await screen.findByRole('textbox', {
      name: `hecate.agentManager.renameInputLabel:${agentWithProvider.name}`,
    });

    fireEvent.click(screen.getByRole('button', { name: 'hecate.agentManager.cancelRename' }));

    await waitFor(() => {
      expect(
        screen.queryByRole('textbox', {
          name: `hecate.agentManager.renameInputLabel:${agentWithProvider.name}`,
        }),
      ).not.toBeInTheDocument();
    });
  });

  it('renders table with all expected column headers', () => {
    renderManager([agentWithProvider]);

    expect(screen.getByRole('columnheader', { name: 'hecate.agentManager.colName' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'hecate.agentManager.colUUID' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'hecate.agentManager.colStatus' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'hecate.agentManager.colProvider' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'hecate.agentManager.colLastSeen' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'hecate.agentManager.colActions', hidden: true })).toBeInTheDocument();
  });

  it('shows device_id in Provider cell when resolved_address is absent', () => {
    renderManager([agentWithDeviceIdOnly]);

    expect(screen.getByText('ts-device-abc')).toBeInTheDocument();
  });

  it('shows em dash fallback in Provider cell when no address or device_id', () => {
    renderManager([agentTunnelOnly]);

    expect(screen.getByText('—')).toBeInTheDocument();
  });

  it('inline rename: pressing Enter calls rename mutation', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.editNameLabel:${agentWithProvider.name}`,
      }),
    );

    const input = await screen.findByRole('textbox', {
      name: `hecate.agentManager.renameInputLabel:${agentWithProvider.name}`,
    });

    fireEvent.change(input, { target: { value: 'Renamed Agent' } });
    fireEvent.keyDown(input, { key: 'Enter' });

    expect(mockRename).toHaveBeenCalledWith(
      { uuid: agentWithProvider.uuid, name: 'Renamed Agent' },
      expect.anything(),
    );
  });

  it('inline rename: pressing Escape cancels without calling mutation', async () => {
    renderManager([agentWithProvider]);

    fireEvent.click(
      screen.getByRole('button', {
        name: `hecate.agentManager.editNameLabel:${agentWithProvider.name}`,
      }),
    );

    const input = await screen.findByRole('textbox', {
      name: `hecate.agentManager.renameInputLabel:${agentWithProvider.name}`,
    });

    fireEvent.keyDown(input, { key: 'Escape' });

    expect(mockRename).not.toHaveBeenCalled();

    await waitFor(() => {
      expect(
        screen.queryByRole('textbox', {
          name: `hecate.agentManager.renameInputLabel:${agentWithProvider.name}`,
        }),
      ).not.toBeInTheDocument();
    });
  });
});
