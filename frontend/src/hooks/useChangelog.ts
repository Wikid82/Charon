import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  ackChangelog,
  getChangelogAll,
  getChangelogStatus,
  optInChangelog,
  type ChangelogAckRequest,
} from '../api/changelog';
import { toast } from '../utils/toast';

/**
 * Fetches the current user's "What's New" status (whether to show the
 * post-auth modal, plus the unseen version entries). Polled infrequently —
 * the modal is a one-shot post-login check, not a live feed.
 */
export function useChangelogStatus() {
  return useQuery({
    queryKey: ['changelog-status'],
    queryFn: getChangelogStatus,
    staleTime: 1000 * 60 * 5,
  });
}

/**
 * Fetches the full changelog history for the Settings "browse" modal.
 * Gated on `enabled` so it only fires while that modal is open.
 */
export function useChangelogAll(enabled: boolean) {
  return useQuery({
    queryKey: ['changelog-all'],
    queryFn: getChangelogAll,
    enabled,
  });
}

/**
 * Dismisses the "What's New" modal (temporarily or permanently) and
 * optionally opts the user out of future notifications.
 */
export function useAckChangelog() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: ChangelogAckRequest) => ackChangelog(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['changelog-status'] });
    },
    onError: () => toast.error('Failed to update changelog preferences'),
  });
}

/** Re-enables automatic "What's New" changelog notifications. */
export function useOptInChangelog() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: optInChangelog,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['changelog-status'] });
    },
    onError: () => toast.error('Failed to re-enable update notifications'),
  });
}
