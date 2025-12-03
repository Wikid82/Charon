import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import {
  getProxyHosts,
  createProxyHost,
  updateProxyHost,
  deleteProxyHost,
  bulkUpdateACL,
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
    mutationFn: (opts: { uuid: string; deleteUptime?: boolean } | string) =>
      typeof opts === 'string' ? deleteProxyHost(opts) : (opts.deleteUptime !== undefined ? deleteProxyHost(opts.uuid, opts.deleteUptime) : deleteProxyHost(opts.uuid)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  const bulkUpdateACLMutation = useMutation({
    mutationFn: ({ hostUUIDs, accessListID }: { hostUUIDs: string[]; accessListID: number | null }) =>
      bulkUpdateACL(hostUUIDs, accessListID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEY });
    },
  });

  return {
    hosts: query.data || [],
    loading: query.isLoading,
    isFetching: query.isFetching,
    error: query.error ? (query.error as Error).message : null,
    createHost: createMutation.mutateAsync,
    updateHost: (uuid: string, data: Partial<ProxyHost>) => updateMutation.mutateAsync({ uuid, data }),
    deleteHost: (uuid: string, deleteUptime?: boolean) => deleteMutation.mutateAsync(deleteUptime !== undefined ? { uuid, deleteUptime } : uuid),
    bulkUpdateACL: (hostUUIDs: string[], accessListID: number | null) =>
      bulkUpdateACLMutation.mutateAsync({ hostUUIDs, accessListID }),
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isBulkUpdating: bulkUpdateACLMutation.isPending,
  };
}

export type { ProxyHost };
