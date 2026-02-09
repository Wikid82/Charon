import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { accessListsApi, type CreateAccessListRequest } from '../api/accessLists';
import toast from 'react-hot-toast';

export function useAccessLists() {
  return useQuery({
    queryKey: ['accessLists'],
    queryFn: accessListsApi.list,
  });
}

export function useAccessList(id: number | undefined) {
  return useQuery({
    queryKey: ['accessList', id],
    queryFn: () => accessListsApi.get(id!),
    enabled: !!id,
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
    mutationFn: ({ id, data }: { id: number; data: Partial<CreateAccessListRequest> }) =>
      accessListsApi.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['accessLists'] });
      queryClient.invalidateQueries({ queryKey: ['accessList', variables.id] });
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
    mutationFn: (id: number) => accessListsApi.delete(id),
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
    mutationFn: ({ id, ipAddress }: { id: number; ipAddress: string }) =>
      accessListsApi.testIP(id, ipAddress),
    onError: (error: Error) => {
      toast.error(`Failed to test IP: ${error.message}`);
    },
  });
}
