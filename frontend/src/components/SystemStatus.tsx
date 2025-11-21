import React from 'react';
import { useQuery } from '@tanstack/react-query';
import { checkUpdates } from '../api/system';

const SystemStatus: React.FC = () => {
  // We still query for updates here to keep the cache fresh,
  // but the UI is now handled by NotificationCenter
  useQuery({
    queryKey: ['system-updates'],
    queryFn: checkUpdates,
    staleTime: 1000 * 60 * 60, // 1 hour
  });

  return null;
};

export default SystemStatus;
