import { render, screen, fireEvent, waitFor, act } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { type OrthrusAgent } from '../../../api/orthrus';
import { AgentExternalProxyDialog } from '../AgentExternalProxyDialog';

const mockPatch = vi.fn();
const mockRefetchStatus = vi.fn();
let mockProxyStatus: Record<string, unknown> | undefined = undefined;

vi.mock('../../../hooks/useOrthrus', () => ({
  usePatchAgent: () => ({ mutate: mockPatch, isPending: false }),
  useAgentProxyStatus: () => ({
    data: mockProxyStatus,
    refetch: mockRefetchStatus,
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) =>
      opts?.name ? `${key}:${opts.name}` : key,
  }),
}));

const baseAgent: OrthrusAgent = {
  uuid: 'agent-1',
  name: 'Test Agent',
  status: 'online',
  capabilities: '["proxy"]',
  hecate_tunnel_uuid: undefined,
  resolved_address: undefined,
  device_id: undefined,
  created_at: '2025-01-01T00:00:00Z',
  updated_at: '2025-01-01T00:00:00Z',
  external_proxy_port: 2375,
};

const renderDialog = (agent: OrthrusAgent = baseAgent, open = true, onClose = vi.fn()) =>
  render(<AgentExternalProxyDialog agent={agent} open={open} onClose={onClose} />);

beforeEach(() => {
  mockPatch.mockReset();
  mockRefetchStatus.mockReset();
  mockProxyStatus = undefined;
});

describe('AgentExternalProxyDialog', () => {
  describe('port validation', () => {
    it('shows no error for port 0', () => {
      renderDialog({ ...baseAgent, external_proxy_port: 0 });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('shows no error for a valid port in 1024–65535', () => {
      renderDialog({ ...baseAgent, external_proxy_port: 2375 });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('shows an error when port is changed to a privileged value (1–1023)', () => {
      renderDialog({ ...baseAgent, external_proxy_port: 0 });
      const input = screen.getByRole('spinbutton');
      fireEvent.change(input, { target: { value: '80' } });
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    it('shows an error when port exceeds 65535', () => {
      renderDialog({ ...baseAgent, external_proxy_port: 0 });
      const input = screen.getByRole('spinbutton');
      fireEvent.change(input, { target: { value: '99999' } });
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    it('clears error when port is changed to 0', () => {
      renderDialog({ ...baseAgent, external_proxy_port: 0 });
      const input = screen.getByRole('spinbutton');
      fireEvent.change(input, { target: { value: '80' } });
      expect(screen.getByRole('alert')).toBeInTheDocument();
      fireEvent.change(input, { target: { value: '0' } });
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  describe('handleSave', () => {
    it('calls patch with the current port when valid', () => {
      renderDialog();
      fireEvent.click(screen.getByText('common.save'));
      expect(mockPatch).toHaveBeenCalledWith(
        { uuid: 'agent-1', req: { external_proxy_port: 2375 } },
        expect.any(Object),
      );
    });

    it('does not call patch when port is invalid', () => {
      renderDialog({ ...baseAgent, external_proxy_port: 0 });
      const input = screen.getByRole('spinbutton');
      fireEvent.change(input, { target: { value: '80' } });
      fireEvent.click(screen.getByText('common.save'));
      expect(mockPatch).not.toHaveBeenCalled();
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });

    it('calls refetchStatus and onClose on successful save', async () => {
      const onClose = vi.fn();
      mockPatch.mockImplementation((_args: unknown, { onSuccess }: { onSuccess: () => void }) => {
        onSuccess();
      });
      renderDialog(baseAgent, true, onClose);
      fireEvent.click(screen.getByText('common.save'));
      expect(mockRefetchStatus).toHaveBeenCalled();
      expect(onClose).toHaveBeenCalled();
    });
  });

  describe('offline agent', () => {
    it('shows offline status when agent is not online', () => {
      renderDialog({ ...baseAgent, status: 'offline' });
      const statusNodes = screen.getAllByText('hecate.externalProxy.agentOffline');
      expect(statusNodes.length).toBeGreaterThan(0);
    });
  });

  describe('live proxy status', () => {
    it('shows error state when proxyStatus.error is set', () => {
      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: true,
        configured_port: 2375,
        active: false,
        active_port: 0,
        bind_address: '',
        connection_string: '',
        error: 'bind failed: address already in use',
      };
      renderDialog();
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByText('bind failed: address already in use')).toBeInTheDocument();
    });

    it('shows offline message when proxyStatus.agent_online is false', () => {
      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: false,
        configured_port: 2375,
        active: false,
        active_port: 0,
        bind_address: '',
        connection_string: '',
        error: '',
      };
      renderDialog();
      const offlineMessages = screen.getAllByText('hecate.externalProxy.agentOffline');
      expect(offlineMessages.length).toBeGreaterThan(0);
    });

    it('shows active status with connection string when proxy is active', () => {
      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: true,
        configured_port: 2375,
        active: true,
        active_port: 2375,
        bind_address: '0.0.0.0:2375',
        connection_string: 'tcp://charon:2375',
        error: '',
      };
      renderDialog();
      expect(screen.getByText('hecate.externalProxy.statusActive')).toBeInTheDocument();
      expect(screen.getByText('tcp://charon:2375')).toBeInTheDocument();
    });

    it('shows inactive status when proxy is not active', () => {
      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: true,
        configured_port: 2375,
        active: false,
        active_port: 0,
        bind_address: '',
        connection_string: '',
        error: '',
      };
      renderDialog();
      expect(screen.getByText('hecate.externalProxy.statusInactive')).toBeInTheDocument();
    });

    it('shows reconnect notice when configured port differs from active port', () => {
      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: true,
        configured_port: 2375,
        active: true,
        active_port: 9999,
        bind_address: '0.0.0.0:9999',
        connection_string: 'tcp://charon:9999',
        error: '',
      };
      renderDialog({ ...baseAgent, external_proxy_port: 2375 });
      expect(screen.getByText('hecate.externalProxy.reconnectNotice')).toBeInTheDocument();
    });
  });

  describe('handleCopy', () => {
    it('copies connection string to clipboard and shows copied state', async () => {
      const writeText = vi.fn().mockResolvedValue(undefined);
      Object.assign(navigator, { clipboard: { writeText } });

      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: true,
        configured_port: 2375,
        active: true,
        active_port: 2375,
        bind_address: '0.0.0.0:2375',
        connection_string: 'tcp://charon:2375',
        error: '',
      };
      renderDialog();

      const copyButton = screen.getByRole('button', {
        name: 'hecate.externalProxy.copyConnectionString',
      });
      await act(async () => {
        fireEvent.click(copyButton);
      });

      await waitFor(() =>
        expect(writeText).toHaveBeenCalledWith('tcp://charon:2375'),
      );
    });
  });

  describe('cancel button', () => {
    it('calls onClose when cancel is clicked', () => {
      const onClose = vi.fn();
      renderDialog(baseAgent, true, onClose);
      fireEvent.click(screen.getByText('common.cancel'));
      expect(onClose).toHaveBeenCalled();
    });
  });
});
