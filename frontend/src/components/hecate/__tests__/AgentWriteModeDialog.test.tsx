import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { toast } from 'react-hot-toast';
import { MemoryRouter } from 'react-router-dom';
import { describe, it, expect, vi, beforeEach } from 'vitest';

import { type OrthrusAgent } from '../../../api/orthrus';
import { AgentWriteModeDialog } from '../AgentWriteModeDialog';

const mockPatch = vi.fn();
let mockProxyStatus: Record<string, unknown> | undefined = undefined;

vi.mock('../../../hooks/useOrthrus', () => ({
  usePatchAgent: () => ({ mutate: mockPatch, isPending: false }),
  useAgentProxyStatus: () => ({ data: mockProxyStatus }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, string>) =>
      opts?.name ? `${key}:${opts.name}` : key,
  }),
}));

vi.mock('react-hot-toast', () => ({
  toast: {
    success: vi.fn(),
  },
}));

const mockToast = vi.mocked(toast);

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
  write_enabled: false,
};

const renderDialog = (agent: OrthrusAgent = baseAgent, open = true, onClose = vi.fn()) =>
  render(
    <MemoryRouter>
      <AgentWriteModeDialog agent={agent} open={open} onClose={onClose} />
    </MemoryRouter>,
  );

beforeEach(() => {
  mockPatch.mockReset();
  mockToast.success.mockReset();
  mockProxyStatus = undefined;
});

describe('AgentWriteModeDialog', () => {
  describe('off by default', () => {
    it('toggle reflects write_enabled: false and no confirmation input is shown', () => {
      renderDialog();
      expect(screen.getByRole('switch')).not.toBeChecked();
      expect(screen.queryByLabelText(/confirmPrompt/)).not.toBeInTheDocument();
    });

  });

  describe('enabling requires typed confirmation', () => {
    it('reveals the confirmation input when toggled on', () => {
      renderDialog();
      fireEvent.click(screen.getByRole('switch'));
      expect(screen.getByLabelText('hecate.writeMode.confirmPrompt:Test Agent')).toBeInTheDocument();
    });

    it('Save is disabled with empty or mismatched confirmation text', () => {
      renderDialog();
      fireEvent.click(screen.getByRole('switch'));

      const saveButton = screen.getByRole('button', { name: 'common.save' });
      expect(saveButton).toBeDisabled();

      const input = screen.getByLabelText('hecate.writeMode.confirmPrompt:Test Agent');
      fireEvent.change(input, { target: { value: 'wrong-name' } });
      expect(saveButton).toBeDisabled();
    });

    it('Save is enabled once the typed value exactly matches the agent name', () => {
      renderDialog();
      fireEvent.click(screen.getByRole('switch'));

      const input = screen.getByLabelText('hecate.writeMode.confirmPrompt:Test Agent');
      fireEvent.change(input, { target: { value: 'Test Agent' } });

      expect(screen.getByRole('button', { name: 'common.save' })).toBeEnabled();
    });

    it('submits write_enabled: true once confirmed', async () => {
      mockPatch.mockImplementation(
        (_args: unknown, { onSuccess }: { onSuccess: (agent: OrthrusAgent) => void }) => {
          onSuccess({ ...baseAgent, write_enabled: true, status: 'offline' });
        },
      );
      const onClose = vi.fn();
      renderDialog(baseAgent, true, onClose);

      fireEvent.click(screen.getByRole('switch'));
      fireEvent.change(screen.getByLabelText('hecate.writeMode.confirmPrompt:Test Agent'), {
        target: { value: 'Test Agent' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'common.save' }));

      await waitFor(() => {
        expect(mockPatch).toHaveBeenCalledWith(
          { uuid: 'agent-1', req: { write_enabled: true } },
          expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
        expect(onClose).toHaveBeenCalled();
      });
    });
  });

  describe('disabling requires no typed confirmation', () => {
    const enabledAgent: OrthrusAgent = { ...baseAgent, write_enabled: true };

    it('toggle starts checked and no confirmation input appears when turning off', () => {
      renderDialog(enabledAgent);
      expect(screen.getByRole('switch')).toBeChecked();

      fireEvent.click(screen.getByRole('switch'));
      expect(screen.queryByLabelText(/confirmPrompt/)).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'hecate.writeMode.disableConfirm' })).toBeEnabled();
    });

    it('submits write_enabled: false without any confirmation text', async () => {
      mockPatch.mockImplementation(
        (_args: unknown, { onSuccess }: { onSuccess: (agent: OrthrusAgent) => void }) => {
          onSuccess({ ...enabledAgent, write_enabled: false, status: 'online' });
        },
      );
      renderDialog(enabledAgent);

      fireEvent.click(screen.getByRole('switch'));
      fireEvent.click(screen.getByRole('button', { name: 'hecate.writeMode.disableConfirm' }));

      await waitFor(() => {
        expect(mockPatch).toHaveBeenCalledWith(
          { uuid: 'agent-1', req: { write_enabled: false } },
          expect.objectContaining({ onSuccess: expect.any(Function) }),
        );
      });
    });
  });

  describe('restart-required toast', () => {
    const patchAndResolve = (updatedAgent: OrthrusAgent) => {
      mockPatch.mockImplementation(
        (_args: unknown, { onSuccess }: { onSuccess: (agent: OrthrusAgent) => void }) => {
          onSuccess(updatedAgent);
        },
      );
    };

    it('fires when turned on while the agent is connected', async () => {
      patchAndResolve({ ...baseAgent, write_enabled: true, status: 'online' });
      renderDialog(baseAgent);

      fireEvent.click(screen.getByRole('switch'));
      fireEvent.change(screen.getByLabelText('hecate.writeMode.confirmPrompt:Test Agent'), {
        target: { value: 'Test Agent' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'common.save' }));

      await waitFor(() => {
        expect(mockToast.success).toHaveBeenCalledWith(
          'hecate.writeMode.restartRequiredToast:Test Agent',
          { id: 'write-mode-restart-agent-1', duration: 8000 },
        );
      });
    });

    it('does not fire when turned on while the agent is disconnected', async () => {
      patchAndResolve({ ...baseAgent, write_enabled: true, status: 'offline' });
      renderDialog(baseAgent);

      fireEvent.click(screen.getByRole('switch'));
      fireEvent.change(screen.getByLabelText('hecate.writeMode.confirmPrompt:Test Agent'), {
        target: { value: 'Test Agent' },
      });
      fireEvent.click(screen.getByRole('button', { name: 'common.save' }));

      await waitFor(() => expect(mockPatch).toHaveBeenCalled());
      expect(mockToast.success).not.toHaveBeenCalled();
    });

    it('does not fire when turned off', async () => {
      const enabledAgent: OrthrusAgent = { ...baseAgent, write_enabled: true };
      patchAndResolve({ ...enabledAgent, write_enabled: false, status: 'online' });
      renderDialog(enabledAgent);

      fireEvent.click(screen.getByRole('switch'));
      fireEvent.click(screen.getByRole('button', { name: 'hecate.writeMode.disableConfirm' }));

      await waitFor(() => expect(mockPatch).toHaveBeenCalled());
      expect(mockToast.success).not.toHaveBeenCalled();
    });

    it('does not fire on a no-op save (already on, saved again unchanged)', async () => {
      const enabledAgent: OrthrusAgent = { ...baseAgent, write_enabled: true };
      patchAndResolve({ ...enabledAgent, write_enabled: true, status: 'online' });
      renderDialog(enabledAgent);

      fireEvent.click(screen.getByRole('button', { name: 'common.save' }));

      await waitFor(() => expect(mockPatch).toHaveBeenCalled());
      expect(mockToast.success).not.toHaveBeenCalled();
    });
  });

  describe('reconnect notice', () => {
    it('is shown when configured write_enabled differs from the live session value', () => {
      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: true,
        configured_port: 2375,
        active: true,
        active_port: 2375,
        bind_address: '0.0.0.0:2375',
        connection_string: 'tcp://charon:2375',
        configured_write_enabled: true,
        active_write_enabled: false,
        error: '',
      };
      renderDialog({ ...baseAgent, write_enabled: true });
      expect(screen.getByText('hecate.writeMode.reconnectNotice')).toBeInTheDocument();
    });

    it('is not shown when configured and active write_enabled agree', () => {
      mockProxyStatus = {
        agent_uuid: 'agent-1',
        agent_online: true,
        configured_port: 2375,
        active: true,
        active_port: 2375,
        bind_address: '0.0.0.0:2375',
        connection_string: 'tcp://charon:2375',
        configured_write_enabled: true,
        active_write_enabled: true,
        error: '',
      };
      renderDialog({ ...baseAgent, write_enabled: true });
      expect(screen.queryByText('hecate.writeMode.reconnectNotice')).not.toBeInTheDocument();
    });
  });

  describe('audit log link', () => {
    it('links to a pre-filtered audit-logs view for this agent', () => {
      renderDialog();
      const link = screen.getByRole('link', { name: 'hecate.writeMode.viewAuditLog' });
      expect(link).toHaveAttribute(
        'href',
        '/audit-logs?resource_uuid=agent-1&event_category=orthrus_write',
      );
    });
  });
});
