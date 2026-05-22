import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';

import { proxyGroupsApi, type CreateProxyGroupRequest } from '../api/proxyGroups';

export const PROXY_GROUPS_QUERY_KEY = ['proxy-groups'];

export function useProxyGroups() {
  return useQuery({
    queryKey: PROXY_GROUPS_QUERY_KEY,
    queryFn: proxyGroupsApi.list,
  });
}

export function useCreateProxyGroup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateProxyGroupRequest) => proxyGroupsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PROXY_GROUPS_QUERY_KEY });
      toast.success('Group created');
    },
    onError: (error: Error) => {
      toast.error(`Failed to create group: ${error.message}`);
    },
  });
}

export function useUpdateProxyGroup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: Partial<CreateProxyGroupRequest> }) =>
      proxyGroupsApi.update(uuid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PROXY_GROUPS_QUERY_KEY });
      toast.success('Group updated');
    },
    onError: (error: Error) => {
      toast.error(`Failed to update group: ${error.message}`);
    },
  });
}

export function useDeleteProxyGroup() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (uuid: string) => proxyGroupsApi.delete(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: PROXY_GROUPS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: ['proxy-hosts'] });
      toast.success('Group deleted — hosts moved to Ungrouped');
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete group: ${error.message}`);
    },
  });
}
