import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getProxyHosts,
  createProxyHost,
  updateProxyHost,
  deleteProxyHost,
  ProxyHost
} from '../api/proxyHosts';

export const QUERY_KEY = ['proxy-hosts'];

export function useProxyHosts() {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: QUERY_KEY,
    queryFn: getProxyHosts,
  });

  const createMutation = useMutation({
    mutationFn: (host: Partial<ProxyHost>) => createProxyHost(host),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: Partial<ProxyHost> }) =>
      updateProxyHost(uuid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => deleteProxyHost(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  return {
    hosts: query.data || [],
    loading: query.isLoading,
    error: query.error ? (query.error as Error).message : null,
    createHost: createMutation.mutateAsync,
    updateHost: (uuid: string, data: Partial<ProxyHost>) => updateMutation.mutateAsync({ uuid, data }),
    deleteHost: deleteMutation.mutateAsync,
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
  };
}

export type { ProxyHost };
