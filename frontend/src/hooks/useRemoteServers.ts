import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getRemoteServers,
  createRemoteServer,
  updateRemoteServer,
  deleteRemoteServer,
  testRemoteServerConnection,
  RemoteServer
} from '../api/remoteServers';

export const QUERY_KEY = ['remote-servers'];

export function useRemoteServers(enabledOnly = false) {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: [...QUERY_KEY, { enabled: enabledOnly }],
    queryFn: () => getRemoteServers(enabledOnly),
  });

  const createMutation = useMutation({
    mutationFn: (server: Partial<RemoteServer>) => createRemoteServer(server),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: Partial<RemoteServer> }) =>
      updateRemoteServer(uuid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => deleteRemoteServer(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const testConnectionMutation = useMutation({
    mutationFn: (uuid: string) => testRemoteServerConnection(uuid),
  });

  return {
    servers: query.data || [],
    loading: query.isLoading,
    error: query.error ? (query.error as Error).message : null,
    createServer: createMutation.mutateAsync,
    updateServer: (uuid: string, data: Partial<RemoteServer>) => updateMutation.mutateAsync({ uuid, data }),
    deleteServer: deleteMutation.mutateAsync,
    testConnection: testConnectionMutation.mutateAsync,
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isTestingConnection: testConnectionMutation.isPending,
  };
}

export type { RemoteServer };
