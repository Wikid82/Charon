import { useQuery } from '@tanstack/react-query';
import { getWebSocketConnections, getWebSocketStats } from '../api/websocket';

/**
 * Hook to fetch and manage WebSocket connection data
 */
export const useWebSocketConnections = () => {
  return useQuery({
    queryKey: ['websocket', 'connections'],
    queryFn: getWebSocketConnections,
    refetchInterval: 5000, // Refresh every 5 seconds
  });
};

/**
 * Hook to fetch and manage WebSocket statistics
 */
export const useWebSocketStats = () => {
  return useQuery({
    queryKey: ['websocket', 'stats'],
    queryFn: getWebSocketStats,
    refetchInterval: 5000, // Refresh every 5 seconds
  });
};
