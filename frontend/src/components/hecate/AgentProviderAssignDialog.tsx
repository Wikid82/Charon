import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

import { NetBirdPeerPicker } from './NetBirdPeerPicker';
import { TailscaleDevicePicker } from './TailscaleDevicePicker';
import { ZeroTierMemberPicker } from './ZeroTierMemberPicker';
import { listTailscaleDevices, type TunnelProvider } from '../../api/hecate';
import { type OrthrusAgent } from '../../api/orthrus';
import { useHecate } from '../../hooks/useHecate';
import { usePatchAgent } from '../../hooks/useOrthrus';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../ui/Dialog';

interface AgentProviderAssignDialogProps {
  agent: OrthrusAgent;
  open: boolean;
  onClose: () => void;
}

export function AgentProviderAssignDialog({
  agent,
  open,
  onClose,
}: AgentProviderAssignDialogProps) {
  const { t } = useTranslation();
  const { tunnels } = useHecate();
  const { mutate: patch, isPending } = usePatchAgent();

  const [selectedTunnelUUID, setSelectedTunnelUUID] = useState(agent.hecate_tunnel_uuid ?? '');
  const [deviceId, setDeviceId] = useState(agent.device_id ?? '');
  const [resolvedAddress, setResolvedAddress] = useState(agent.resolved_address ?? '');
  const [pickerOpen, setPickerOpen] = useState(false);

  const selectedTunnel = tunnels.find((tn) => tn.uuid === selectedTunnelUUID);
  const provider = selectedTunnel?.provider as TunnelProvider | undefined;

  const { data: tailscaleDevices = [] } = useQuery({
    queryKey: ['hecate', 'tailscale', 'devices'],
    queryFn: listTailscaleDevices,
    enabled: pickerOpen && provider === 'tailscale',
    staleTime: 60_000,
  });

  const handleTunnelChange = (uuid: string) => {
    setSelectedTunnelUUID(uuid);
    setDeviceId('');
    setResolvedAddress('');
  };

  const handleSave = () => {
    patch(
      {
        uuid: agent.uuid,
        req: {
          hecate_tunnel_uuid: selectedTunnelUUID || undefined,
          device_id: deviceId || undefined,
          resolved_address: resolvedAddress || undefined,
        },
      },
      { onSuccess: onClose },
    );
  };

  const handleRemove = () => {
    patch(
      {
        uuid: agent.uuid,
        req: {
          hecate_tunnel_uuid: null,
          device_id: null,
          resolved_address: null,
        },
      },
      { onSuccess: onClose },
    );
  };

  const PROVIDERS = ['cloudflare', 'tailscale', 'netbird', 'zerotier'] as const;

  return (
    <Dialog open={open} onOpenChange={onClose}>
      <DialogContent aria-labelledby="assign-provider-title">
        <DialogHeader>
          <DialogTitle id="assign-provider-title">
            {t('hecate.agentManager.assignProviderTitle', { name: agent.name })}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4 py-4">
          {/* Tunnel selector grouped by provider */}
          <div>
            <label
              htmlFor="assign-tunnel"
              className="block text-sm font-medium mb-1 text-content-primary"
            >
              {t('hecate.agentManager.providerTunnel')}
            </label>
            <select
              id="assign-tunnel"
              value={selectedTunnelUUID}
              onChange={(e) => handleTunnelChange(e.target.value)}
              className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">{t('hecate.form.mode.selectProvider')}</option>
              {PROVIDERS.map((p) => {
                const pts = tunnels.filter((tn) => tn.provider === p);
                if (!pts.length) return null;
                return (
                  <optgroup key={p} label={p.charAt(0).toUpperCase() + p.slice(1)}>
                    {pts.map((tn) => (
                      <option key={tn.uuid} value={tn.uuid}>
                        {tn.name}
                      </option>
                    ))}
                  </optgroup>
                );
              })}
            </select>
          </div>

          {/* Cloudflare: hostname input */}
          {selectedTunnel && provider === 'cloudflare' && (
            <div className="space-y-1">
              <label
                htmlFor="assign-cf-hostname"
                className="block text-sm font-medium text-content-primary"
              >
                {t('hecate.form.provider.cloudflareTunnelHostname')}
              </label>
              <input
                id="assign-cf-hostname"
                type="text"
                placeholder="app.example.com"
                value={resolvedAddress}
                onChange={(e) => {
                  setResolvedAddress(e.target.value);
                  setDeviceId('');
                }}
                className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
                aria-describedby="assign-cf-hostname-hint"
              />
              <p id="assign-cf-hostname-hint" className="text-xs text-content-muted">
                {t('hecate.form.provider.cloudflareTunnelHint')}
              </p>
            </div>
          )}

          {/* Non-Cloudflare: device picker trigger */}
          {selectedTunnel && provider !== 'cloudflare' && (
            <div>
              <p className="text-sm font-medium mb-1 text-content-primary">
                {t('hecate.agentManager.deviceId')}
              </p>
              {deviceId && (
                <p className="text-xs font-mono text-content-secondary mb-1">{deviceId}</p>
              )}
              <button
                type="button"
                className="text-sm text-blue-400 hover:text-blue-300 underline focus:outline-none focus:ring-2 focus:ring-blue-500 rounded"
                onClick={() => setPickerOpen(true)}
              >
                {t('hecate.form.mode.selectDevice')}
              </button>
            </div>
          )}

          {/* Resolved address — auto-filled from picker, editable (non-Cloudflare) */}
          {selectedTunnel && provider !== 'cloudflare' && (
            <div>
              <label
                htmlFor="assign-resolved"
                className="block text-sm font-medium mb-1 text-content-primary"
              >
                {t('hecate.agentManager.resolvedAddress')}
              </label>
              <input
                id="assign-resolved"
                type="text"
                value={resolvedAddress}
                onChange={(e) => setResolvedAddress(e.target.value)}
                placeholder="100.x.x.x or hostname"
                className="w-full bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          )}
        </div>

        <DialogFooter>
          {agent.hecate_tunnel_uuid && (
            <button
              type="button"
              onClick={handleRemove}
              disabled={isPending}
                className="px-4 py-2 rounded text-sm text-error hover:text-error border border-error/40 hover:border-error focus:outline-none focus:ring-2 focus:ring-error disabled:opacity-50 mr-auto"
            >
              {t('hecate.agentManager.removeProviderAssignment')}
            </button>
          )}
          <button
            type="button"
            onClick={onClose}
            disabled={isPending}
            className="px-4 py-2 rounded text-sm text-content-secondary hover:text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            {t('common.cancel')}
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={isPending || !selectedTunnelUUID}
            className="px-4 py-2 rounded bg-blue-600 text-white text-sm hover:bg-blue-500 focus:outline-none focus:ring-2 focus:ring-blue-500 disabled:opacity-50"
          >
            {t('hecate.agentManager.saveProviderAssignment')}
          </button>
        </DialogFooter>

        {/* Provider-specific device pickers */}
        {pickerOpen && provider === 'tailscale' && (
          <TailscaleDevicePicker
            open
            devices={tailscaleDevices}
            onClose={() => setPickerOpen(false)}
            onSelect={(device) => {
              setDeviceId(device.id);
              setResolvedAddress(device.addresses[0] ?? '');
              setPickerOpen(false);
            }}
            selectedId={deviceId}
          />
        )}
        {pickerOpen && provider === 'netbird' && (
          <NetBirdPeerPicker
            open
            onClose={() => setPickerOpen(false)}
            onSelect={(peer) => {
              setDeviceId(peer.id);
              setResolvedAddress(peer.ip);
              setPickerOpen(false);
            }}
            selectedId={deviceId}
          />
        )}
        {pickerOpen && provider === 'zerotier' && (
          <ZeroTierMemberPicker
            open
            onClose={() => setPickerOpen(false)}
            onSelect={(member) => {
              setDeviceId(member.node_id);
              setResolvedAddress(member.ip_assignments[0] ?? '');
              setPickerOpen(false);
            }}
            selectedId={deviceId}
          />
        )}
      </DialogContent>
    </Dialog>
  );
}
