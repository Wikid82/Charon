import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  createTunnel,
  deleteTunnel,
  getTunnelStatus,
  listTunnels,
  rotateCredentials,
  startTunnel,
  stopTunnel,
  updateTunnel,
  type CreateTunnelRequest,
  type TunnelConfig,
  type TunnelStatus,
  type UpdateTunnelRequest,
} from '../api/hecate';

export const TUNNELS_QUERY_KEY = ['hecate', 'tunnels'] as const;
export const STATUS_QUERY_KEY = ['hecate', 'status'] as const;

export const useHecate = () => {
  const queryClient = useQueryClient();

  const {
    data: tunnels = [],
    isLoading: loadingTunnels,
    error: tunnelsError,
  } = useQuery<TunnelConfig[]>({
    queryKey: TUNNELS_QUERY_KEY,
    queryFn: listTunnels,
  });

  const {
    data: statuses = [],
    isLoading: loadingStatus,
    error: statusError,
  } = useQuery<TunnelStatus[]>({
    queryKey: STATUS_QUERY_KEY,
    queryFn: getTunnelStatus,
    refetchInterval: (query) =>
      query.state.data?.some((s) => s.state === 'connecting') ? 10_000 : false,
  });

  const createMutation = useMutation({
    mutationFn: (req: CreateTunnelRequest) => createTunnel(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY });
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ uuid, req }: { uuid: string; req: UpdateTunnelRequest }) =>
      updateTunnel(uuid, req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_QUERY_KEY });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (uuid: string) => deleteTunnel(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: TUNNELS_QUERY_KEY });
      queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY });
    },
  });

  const startMutation = useMutation({
    mutationFn: (uuid: string) => startTunnel(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY });
    },
  });

  const stopMutation = useMutation({
    mutationFn: (uuid: string) => stopTunnel(uuid),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: STATUS_QUERY_KEY });
    },
  });

  const rotateMutation = useMutation({
    mutationFn: ({ uuid, credentials }: { uuid: string; credentials: string }) =>
      rotateCredentials(uuid, credentials),
  });

  const getStatus = (uuid: string): TunnelStatus | undefined =>
    statuses.find((s) => s.uuid === uuid);

  return {
    tunnels,
    statuses,
    loadingTunnels,
    loadingStatus,
    error: tunnelsError?.message ?? statusError?.message ?? null,
    tunnelsError,
    statusError,
    getStatus,
    createTunnel: createMutation.mutateAsync,
    updateTunnel: updateMutation.mutateAsync,
    deleteTunnel: deleteMutation.mutateAsync,
    startTunnel: startMutation.mutateAsync,
    stopTunnel: stopMutation.mutateAsync,
    rotateCredentials: rotateMutation.mutateAsync,
    isCreating: createMutation.isPending,
    isUpdating: updateMutation.isPending,
    isDeleting: deleteMutation.isPending,
    isStarting: startMutation.isPending,
    isStopping: stopMutation.isPending,
    isRotating: rotateMutation.isPending,
  };
};
