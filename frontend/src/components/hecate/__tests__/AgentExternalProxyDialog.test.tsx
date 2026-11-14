import { render, screen, fireEvent, waitFor, within } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { type LocalNetworks, type OrthrusAgent } from '../../../api/orthrus';
import { AgentExternalProxyDialog } from '../AgentExternalProxyDialog';

const mockPatch = vi.fn();
const mockRefetchStatus = vi.fn();
let mockProxyStatus: Record<string, unknown> | undefined;
let mockLocalNetworks: LocalNetworks | undefined;

vi.mock('../../../hooks/useOrthrus', () => ({
  usePatchAgent: () => ({ mutate: mockPatch, isPending: false }),
  useAgentProxyStatus: () => ({
    data: mockProxyStatus,
    refetch: mockRefetchStatus,
  }),
  useLocalNetworks: () => ({ data: mockLocalNetworks }),
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
  external_proxy_bind: '',
  write_enabled: false,
};

const detectedNetworks: LocalNetworks = {
  auto: { address: '172.18.0.4', source: 'default_route' },
  networks: [
    { interface: 'eth0', address: '172.18.0.4', cidr: '172.18.0.0/16', primary: true },
  ],
};

const renderDialog = (agent: OrthrusAgent = baseAgent, open = true, onClose = vi.fn()) =>
  render(<AgentExternalProxyDialog agent={agent} open={open} onClose={onClose} />);

beforeEach(() => {
  mockPatch.mockReset();
  mockRefetchStatus.mockReset();
  mockProxyStatus = undefined;
  mockLocalNetworks = detectedNetworks;
});

const activeStatus = {
  agent_uuid: 'agent-1',
  agent_online: true,
  configured_port: 2375,
  active: true,
  active_port: 2375,
  bind_address: '0.0.0.0:2375',
  configured_bind: '',
  active_bind: '',
  bind_mode: 'auto',
  bind_fallback: false,
  configured_write_enabled: false,
  active_write_enabled: false,
  connection_string: 'tcp://charon:2375',
  error: '',
};

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
        { uuid: 'agent-1', req: { external_proxy_port: 2375, external_proxy_bind: '' } },
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

  describe('listen on setting', () => {
    const bindSelect = () => screen.getByLabelText('hecate.externalProxy.bindLabel');

    it('offers automatic, each detected network, all networks and custom', () => {
      renderDialog();
      const options = within(bindSelect()).getAllByRole('option').map((o) => o.textContent);
      expect(options).toEqual([
        'hecate.externalProxy.bindAuto',
        'eth0 · 172.18.0.4 (172.18.0.0/16)',
        'hecate.externalProxy.bindAll',
        'hecate.externalProxy.bindCustom',
      ]);
    });

    it('saves all networks as 0.0.0.0 and shows the passive exposure note instead of the generic warning', () => {
      renderDialog();
      fireEvent.change(bindSelect(), { target: { value: 'all' } });

      expect(screen.getByText('hecate.externalProxy.bindAllNote')).toBeInTheDocument();
      expect(screen.queryByText('hecate.externalProxy.securityWarning')).not.toBeInTheDocument();

      fireEvent.click(screen.getByText('common.save'));
      expect(mockPatch).toHaveBeenCalledWith(
        { uuid: 'agent-1', req: { external_proxy_port: 2375, external_proxy_bind: '0.0.0.0' } },
        expect.any(Object),
      );
    });

    it('saves a detected network as its CIDR', () => {
      renderDialog();
      fireEvent.change(bindSelect(), { target: { value: 'net:172.18.0.0/16' } });

      fireEvent.click(screen.getByText('common.save'));
      expect(mockPatch).toHaveBeenCalledWith(
        { uuid: 'agent-1', req: { external_proxy_port: 2375, external_proxy_bind: '172.18.0.0/16' } },
        expect.any(Object),
      );
    });

    it('saves a typed custom address and disables save while the custom value is blank', () => {
      renderDialog();
      fireEvent.change(bindSelect(), { target: { value: 'custom' } });
      const custom = screen.getByLabelText('hecate.externalProxy.bindCustomLabel');

      fireEvent.change(custom, { target: { value: '   ' } });
      expect(screen.getByText('common.save')).toBeDisabled();

      fireEvent.change(custom, { target: { value: '10.0.0.5' } });
      expect(screen.getByText('common.save')).not.toBeDisabled();
      fireEvent.click(screen.getByText('common.save'));
      expect(mockPatch).toHaveBeenCalledWith(
        { uuid: 'agent-1', req: { external_proxy_port: 2375, external_proxy_bind: '10.0.0.5' } },
        expect.any(Object),
      );
    });

    it('seeds the select from a stored subnet', () => {
      renderDialog({ ...baseAgent, external_proxy_bind: '172.18.0.0/16' });
      expect(bindSelect()).toHaveValue('net:172.18.0.0/16');
    });

    it('explains loopback as not reachable from other containers or hosts when automatic resolves to it', () => {
      mockLocalNetworks = {
        auto: { address: '127.0.0.1', source: 'loopback_fallback' },
        networks: [],
      };
      renderDialog();
      expect(screen.getByText('hecate.externalProxy.loopbackNote')).toBeInTheDocument();
      expect(screen.queryByText(/this server only/i)).not.toBeInTheDocument();
    });

    it('shows the fallback note when the live session fell back to loopback', () => {
      mockProxyStatus = {
        ...activeStatus,
        active: true,
        active_bind: '',
        bind_address: '127.0.0.1:2375',
        bind_fallback: true,
      };
      renderDialog();
      expect(screen.getByText('hecate.externalProxy.fallbackNote')).toBeInTheDocument();
    });

    it('shows the reconnect notice when the saved listen setting differs from the live one', () => {
      mockProxyStatus = {
        ...activeStatus,
        configured_bind: '172.18.0.0/16',
        active_bind: '',
      };
      renderDialog();
      expect(screen.getByText('hecate.externalProxy.bindReconnectNotice')).toBeInTheDocument();
      expect(screen.queryByText('hecate.externalProxy.reconnectNotice')).not.toBeInTheDocument();
    });

    it('shows the port reconnect notice only when the port diverges', () => {
      mockProxyStatus = { ...activeStatus, configured_bind: '', active_bind: '', active_port: 9999 };
      renderDialog({ ...baseAgent, external_proxy_port: 2375 });
      expect(screen.getByText('hecate.externalProxy.reconnectNotice')).toBeInTheDocument();
      expect(screen.queryByText('hecate.externalProxy.bindReconnectNotice')).not.toBeInTheDocument();
    });

    it('shows the bind-invalid text when the server refuses the listen setting', () => {
      mockPatch.mockImplementation((_args: unknown, { onError }: { onError: (err: unknown) => void }) => {
        onError(
          Object.assign(new Error('bad'), {
            isAxiosError: true,
            response: { data: { error_code: 'external_proxy_bind_invalid' } },
          }),
        );
      });
      renderDialog();
      fireEvent.click(screen.getByText('common.save'));
      expect(screen.getByRole('alert')).toHaveTextContent('hecate.externalProxy.bindInvalid');
    });

    it('shows the generic save error for other failures', () => {
      mockPatch.mockImplementation((_args: unknown, { onError }: { onError: (err: unknown) => void }) => {
        onError(new Error('server down'));
      });
      renderDialog();
      fireEvent.click(screen.getByText('common.save'));
      expect(screen.getByRole('alert')).toHaveTextContent('hecate.externalProxy.saveError');
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
        configured_write_enabled: false,
        active_write_enabled: false,
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
        configured_write_enabled: false,
        active_write_enabled: false,
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
        configured_write_enabled: false,
        active_write_enabled: false,
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
        configured_write_enabled: false,
        active_write_enabled: false,
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
        configured_write_enabled: false,
        active_write_enabled: false,
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
        configured_write_enabled: false,
        active_write_enabled: false,
        connection_string: 'tcp://charon:2375',
        error: '',
      };
      renderDialog();

      const copyButton = screen.getByRole('button', {
        name: 'hecate.externalProxy.copyConnectionString',
      });
      fireEvent.click(copyButton);

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

describe('AgentExternalProxyDialog re-seeding', () => {
  const dialog = (agent: OrthrusAgent, open: boolean) => (
    <AgentExternalProxyDialog agent={agent} open={open} onClose={vi.fn()} />
  );

  it('resets an edited port and clears the error when reopened', () => {
    const { rerender } = render(dialog(baseAgent, true));
    fireEvent.change(screen.getByRole('spinbutton'), { target: { value: '80' } });
    expect(screen.getByRole('alert')).toBeInTheDocument();

    rerender(dialog(baseAgent, false));
    rerender(dialog(baseAgent, true));
    expect(screen.getByRole('spinbutton')).toHaveValue(2375);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('re-seeds the port when the agent external_proxy_port changes while open', () => {
    const { rerender } = render(dialog(baseAgent, true));
    fireEvent.change(screen.getByRole('spinbutton'), { target: { value: '80' } });
    rerender(dialog({ ...baseAgent, external_proxy_port: 3000 }, true));
    expect(screen.getByRole('spinbutton')).toHaveValue(3000);
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('does not re-seed while closed', () => {
    const { rerender } = render(dialog(baseAgent, false));
    rerender(dialog({ ...baseAgent, external_proxy_port: 3000 }, false));
    rerender(dialog({ ...baseAgent, external_proxy_port: 4000 }, true));
    expect(screen.getByRole('spinbutton')).toHaveValue(4000);
  });
});
