import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';

import { redirectionHostsApi, type RedirectionHost } from '../api/redirectionHosts';

export const REDIRECTION_HOSTS_QUERY_KEY = ['redirection-hosts'];

export function useRedirectionHosts() {
  return useQuery({
    queryKey: REDIRECTION_HOSTS_QUERY_KEY,
    queryFn: redirectionHostsApi.list,
  });
}

export function useCreateRedirectionHost() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: Partial<RedirectionHost>) => redirectionHostsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: REDIRECTION_HOSTS_QUERY_KEY });
      toast.success('Redirection host created');
    },
    onError: (error: Error) => {
      toast.error(`Failed to create redirection host: ${error.message}`);
    },
  });
}

export function useUpdateRedirectionHost() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: Partial<RedirectionHost> }) =>
      redirectionHostsApi.update(uuid, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: REDIRECTION_HOSTS_QUERY_KEY });
      toast.success('Redirection host updated');
    },
    onError: (error: Error) => {
      toast.error(`Failed to update redirection host: ${error.message}`);
    },
  });
}

export function useDeleteRedirectionHost() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (uuid: string) => redirectionHostsApi.delete(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: REDIRECTION_HOSTS_QUERY_KEY });
      toast.success('Redirection host deleted');
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete redirection host: ${error.message}`);
    },
  });
}
