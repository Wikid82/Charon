import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';

import { accessListsApi, type CreateAccessListRequest } from '../api/accessLists';

export function useAccessLists() {
  return useQuery({
    queryKey: ['accessLists'],
    queryFn: accessListsApi.list,
  });
}

export function useAccessList(uuid: string | undefined) {
  return useQuery({
    queryKey: ['accessList', uuid],
    queryFn: () => accessListsApi.get(uuid!),
    enabled: !!uuid,
  });
}

export function useAccessListTemplates() {
  return useQuery({
    queryKey: ['accessListTemplates'],
    queryFn: accessListsApi.getTemplates,
  });
}

export function useCreateAccessList() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateAccessListRequest) => accessListsApi.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accessLists'] });
      toast.success('Access list created successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to create access list: ${error.message}`);
    },
  });
}

export function useUpdateAccessList() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ uuid, data }: { uuid: string; data: Partial<CreateAccessListRequest> }) =>
      accessListsApi.update(uuid, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['accessLists'] });
      queryClient.invalidateQueries({ queryKey: ['accessList', variables.uuid] });
      toast.success('Access list updated successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to update access list: ${error.message}`);
    },
  });
}

export function useDeleteAccessList() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (uuid: string) => accessListsApi.delete(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['accessLists'] });
      toast.success('Access list deleted successfully');
    },
    onError: (error: Error) => {
      toast.error(`Failed to delete access list: ${error.message}`);
    },
  });
}

export function useTestIP() {
  return useMutation({
    mutationFn: ({ uuid, ipAddress }: { uuid: string; ipAddress: string }) =>
      accessListsApi.testIP(uuid, ipAddress),
    onError: (error: Error) => {
      toast.error(`Failed to test IP: ${error.message}`);
    },
  });
}
