import { Monitor, Wifi } from 'lucide-react';
import { useTranslation } from 'react-i18next';

import { type TailscaleDevice } from '../../api/hecate';
import { cn } from '../../utils/cn';
import { Badge } from '../ui/Badge';
import { Button } from '../ui/Button';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '../ui/Dialog';

interface TailscaleDevicePickerProps {
  devices: TailscaleDevice[];
  open: boolean;
  onClose: () => void;
  onSelect: (device: TailscaleDevice) => void;
  selectedId?: string;
}

export const TailscaleDevicePicker = ({
  devices,
  open,
  onClose,
  onSelect,
  selectedId,
}: TailscaleDevicePickerProps) => {
  const { t } = useTranslation();

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t('hecate.tailscale.pickerTitle')}</DialogTitle>
          <DialogDescription>{t('hecate.tailscale.pickerDescription')}</DialogDescription>
        </DialogHeader>

        <div className="px-6 pb-6 space-y-2 max-h-96 overflow-y-auto">
          {devices.length === 0 ? (
            <p className="text-sm text-content-muted text-center py-8">
              {t('hecate.tailscale.noDevices')}
            </p>
          ) : (
            <ul role="listbox" aria-label={t('hecate.tailscale.pickerTitle')}>
              {devices.map((device) => (
                <li key={device.id}>
                  <Button
                    variant="ghost"
                    role="option"
                    aria-selected={device.id === selectedId}
                    className={cn(
                      'w-full justify-start gap-3 h-auto py-3 px-4 rounded-lg',
                      device.id === selectedId && 'bg-brand-500/10 text-brand-500',
                    )}
                    onClick={() => {
                      onSelect(device);
                      onClose();
                    }}
                  >
                    <Monitor className="h-4 w-4 shrink-0" aria-hidden="true" />
                    <div className="flex-1 text-left min-w-0">
                      <p className="font-medium text-sm truncate">{device.hostname}</p>
                      <p className="text-xs text-content-muted truncate">
                        {device.addresses[0] ?? ''}
                      </p>
                    </div>
                    <div className="flex items-center gap-2 shrink-0">
                      <Badge
                        variant={device.online ? 'success' : 'secondary'}
                        size="sm"
                        className="gap-1"
                      >
                        <Wifi className="h-3 w-3" aria-hidden="true" />
                        {device.online ? t('common.online') : t('common.offline')}
                      </Badge>
                      <span className="text-xs text-content-muted">{device.os}</span>
                    </div>
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
};
