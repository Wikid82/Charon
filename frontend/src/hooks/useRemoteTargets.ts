import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'

import {
  getRemoteTargets,
  createRemoteTarget,
  updateRemoteTarget,
  deleteRemoteTarget,
  testRemoteTarget,
  testDraftRemoteTarget,
  type RemoteTarget,
  type RemoteTargetConfig,
  type RemoteTargetSecrets,
  type RemoteTargetPayload,
  type TestRemoteTargetResponse,
  type TestDraftRemoteTargetPayload,
} from '../api/backups'

/** Query key for the remote storage target list (spec §3.8). */
export const REMOTE_TARGETS_QUERY_KEY = ['backup-remote-targets']

/** Fetches all configured remote storage targets. */
export function useRemoteTargets() {
  return useQuery({
    queryKey: REMOTE_TARGETS_QUERY_KEY,
    queryFn: getRemoteTargets,
  })
}

/** Creates a new remote storage target; invalidates the target list on success. */
export function useCreateRemoteTarget() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (payload: RemoteTargetPayload) => createRemoteTarget(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: REMOTE_TARGETS_QUERY_KEY })
    },
  })
}

/** Updates a remote storage target; invalidates the target list on success. */
export function useUpdateRemoteTarget() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: ({ uuid, payload }: { uuid: string; payload: Partial<RemoteTargetPayload> }) =>
      updateRemoteTarget(uuid, payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: REMOTE_TARGETS_QUERY_KEY })
    },
  })
}

/**
 * Deletes a remote storage target. Removes it from the cached list directly
 * (rather than only invalidating) so the row disappears immediately without
 * depending on a background refetch.
 */
export function useDeleteRemoteTarget() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (uuid: string) => deleteRemoteTarget(uuid),
    onSuccess: (_, uuid) => {
      queryClient.setQueryData<RemoteTarget[]>(REMOTE_TARGETS_QUERY_KEY, (old) =>
        old ? old.filter((target) => target.uuid !== uuid) : old
      )
    },
  })
}

/** Tests connectivity/auth for a remote storage target (no invalidation — read-only probe). */
export function useTestRemoteTarget() {
  return useMutation({
    mutationFn: (uuid: string) => testRemoteTarget(uuid),
  })
}

/**
 * Runs stateless SFTP host-key discovery for a draft (unsaved, no UUID yet)
 * remote target. No invalidation — read-only probe, nothing persists.
 */
export function useTestDraftRemoteTarget() {
  return useMutation({
    mutationFn: (payload: TestDraftRemoteTargetPayload) => testDraftRemoteTarget(payload),
  })
}

export type {
  RemoteTarget,
  RemoteTargetConfig,
  RemoteTargetSecrets,
  RemoteTargetPayload,
  TestRemoteTargetResponse,
  TestDraftRemoteTargetPayload,
}
