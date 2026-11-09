import { useSystemUpdates } from '../hooks/useSystemUpdates';

import type React from 'react';

const SystemStatus: React.FC = () => {
  // We still query for updates here to keep the cache fresh,
  // but the UI is now handled by NotificationCenter
  useSystemUpdates();

  return null;
};

export default SystemStatus;
