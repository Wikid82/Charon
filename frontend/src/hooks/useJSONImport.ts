import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import {
  uploadJSONExport,
  commitJSONImport,
  cancelJSONImport,
  JSONImportPreview,
  JSONImportCommitResult,
} from '../api/jsonImport';

/**
 * Hook for managing JSON import workflow.
 * Provides upload, commit, and cancel functionality with state management.
 */
export function useJSONImport() {
  const queryClient = useQueryClient();
  const [preview, setPreview] = useState<JSONImportPreview | null>(null);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [commitResult, setCommitResult] = useState<JSONImportCommitResult | null>(null);

  const uploadMutation = useMutation({
    mutationFn: uploadJSONExport,
    onSuccess: (data) => {
      setPreview(data);
      setSessionId(data.session.id);
    },
  });

  const commitMutation = useMutation({
    mutationFn: ({
      resolutions,
      names,
    }: {
      resolutions: Record<string, string>;
      names: Record<string, string>;
    }) => {
      if (!sessionId) throw new Error('No active session');
      return commitJSONImport(sessionId, resolutions, names);
    },
    onSuccess: (data) => {
      setCommitResult(data);
      setPreview(null);
      setSessionId(null);
      queryClient.invalidateQueries({ queryKey: ['proxy-hosts'] });
    },
  });

  const cancelMutation = useMutation({
    mutationFn: cancelJSONImport,
    onSuccess: () => {
      setPreview(null);
      setSessionId(null);
    },
  });

  const clearCommitResult = () => {
    setCommitResult(null);
  };

  const reset = () => {
    setPreview(null);
    setSessionId(null);
    setCommitResult(null);
  };

  return {
    preview,
    sessionId,
    loading: uploadMutation.isPending,
    error: uploadMutation.error,
    upload: uploadMutation.mutateAsync,
    commit: (resolutions: Record<string, string>, names: Record<string, string>) =>
      commitMutation.mutateAsync({ resolutions, names }),
    committing: commitMutation.isPending,
    commitError: commitMutation.error,
    commitResult,
    clearCommitResult,
    cancel: cancelMutation.mutateAsync,
    cancelling: cancelMutation.isPending,
    reset,
  };
}

export type { JSONImportPreview, JSONImportCommitResult };
