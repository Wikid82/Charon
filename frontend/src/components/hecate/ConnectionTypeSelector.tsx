import { useTranslation } from 'react-i18next';

import { NativeSelect } from '../ui/NativeSelect';

export type ConnectionType = 'direct' | 'orthrus' | 'cloudflare';

interface ConnectionTypeSelectorProps {
  value: ConnectionType;
  onChange: (value: ConnectionType) => void;
  id?: string;
  disabled?: boolean;
}

export const ConnectionTypeSelector = ({
  value,
  onChange,
  id = 'connection-type',
  disabled,
}: ConnectionTypeSelectorProps) => {
  const { t } = useTranslation();

  return (
    <NativeSelect
      id={id}
      value={value}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value as ConnectionType)}
      aria-label={t('remoteServers.connectionType')}
    >
      <option value="direct">{t('remoteServers.connectionTypeDirect')}</option>
      <option value="orthrus">{t('remoteServers.connectionTypeOrthrus')}</option>
      <option value="cloudflare">{t('remoteServers.connectionTypeCloudflare')}</option>
    </NativeSelect>
  );
};
