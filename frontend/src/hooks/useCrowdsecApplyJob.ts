import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { isAxiosError } from 'axios'
import { useCallback, useEffect, useRef, useState } from 'react'

import { isCrowdsecBusyError } from '../api/errorBody'
import {
  applyPresetAsync,
  cancelApplyJob,
  getApplyJob,
  getApplyJobErrorCode,
  getCrowdsecBusyJobId,
  getCurrentApplyJob,
  type CrowdsecApplyJob,
} from '../api/presets'
import { isApplyJobTerminal } from '../utils/crowdsecApplyJob'

export const APPLY_JOB_QUERY_KEY = ['crowdsec-apply-job'] as const

const FAST_POLL_MS = 2_000
const MEDIUM_POLL_MS = 5_000
const SLOW_POLL_MS = 10_000
const FAST_POLL_WINDOW_S = 30
const MEDIUM_POLL_WINDOW_S = 300
const MAX_BACKOFF_MS = 30_000

/**
 * Delay before the next status poll: 2 s for the first 30 s of the job, 5 s until 5 minutes, then 10 s.
 * After failed polls the delay doubles from 2 s per consecutive failure, capped at 30 s.
 */
export function applyJobPollDelayMs(elapsedSeconds: number, consecutiveFailures: number): number {
  const steady = elapsedSeconds < FAST_POLL_WINDOW_S ? FAST_POLL_MS : elapsedSeconds < MEDIUM_POLL_WINDOW_S ? MEDIUM_POLL_MS : SLOW_POLL_MS
  return consecutiveFailures > 0 ? Math.min(FAST_POLL_MS * 2 ** (consecutiveFailures - 1), MAX_BACKOFF_MS) : steady
}

export interface ApplyJobRequest {
  slug: string
  type?: string
  cache_key?: string
}

interface TrackedJob {
  /** Job being followed; null until one starts or is found on load. */
  id: string | null
  /** Finished job the user already has dealt with; it is not shown again. */
  dismissedId: string | null
}

/**
 * Follows the background hub preset install: resumes an active (or just finished) job on mount,
 * polls it until a terminal state with a growing delay, and survives poll failures.
 * Navigating away does not stop the job; the next mount picks it up again.
 */
export function useCrowdsecApplyJob(options: { enabled?: boolean } = {}) {
  const enabled = options.enabled ?? true
  const queryClient = useQueryClient()
  const [tracked, setTracked] = useState<TrackedJob>({ id: null, dismissedId: null })
  const finishedRef = useRef<string | null>(null)
  // Consecutive failed polls per job id; the query's own failure count restarts with every fetch.
  const failuresRef = useRef<{ id: string | null; count: number }>({ id: null, count: 0 })

  const currentQuery = useQuery({
    queryKey: [...APPLY_JOB_QUERY_KEY, 'current'],
    queryFn: getCurrentApplyJob,
    enabled,
    retry: false,
    refetchOnWindowFocus: false,
  })
  const current = currentQuery.data ?? null
  const resumedJob = current && current.job_id !== tracked.dismissedId ? current : null
  const jobId = tracked.id ?? resumedJob?.job_id ?? null
  const seed = resumedJob && resumedJob.job_id === jobId ? resumedJob : undefined

  const jobQuery = useQuery({
    queryKey: [...APPLY_JOB_QUERY_KEY, 'job', jobId],
    queryFn: async () => {
      const id = jobId as string
      try {
        const fresh = await getApplyJob(id)
        failuresRef.current = { id, count: 0 }
        return fresh
      } catch (err) {
        failuresRef.current = { id, count: failuresRef.current.id === id ? failuresRef.current.count + 1 : 1 }
        throw err
      }
    },
    enabled: enabled && jobId !== null && !(seed && isApplyJobTerminal(seed)),
    retry: false,
    refetchOnWindowFocus: false,
    refetchInterval: (query) => {
      const data = query.state.data
      const gone = getApplyJobErrorCode(query.state.error) === 'job_not_found'
      const done = (data !== undefined && isApplyJobTerminal(data)) || gone
      return done ? false : applyJobPollDelayMs(data?.elapsed_seconds ?? 0, failuresRef.current.count)
    },
  })

  // A job that vanished (pruned) has nothing left to show.
  const jobGone = jobQuery.isError && getApplyJobErrorCode(jobQuery.error) === 'job_not_found'
  const job: CrowdsecApplyJob | null = jobId === null || jobGone ? null : (jobQuery.data ?? seed ?? null)
  const isActive = job !== null && !isApplyJobTerminal(job)
  // A failed poll keeps the last known job; the row says the server is not answering until one succeeds.
  const isWaitingForServer = isActive && jobQuery.isError

  // The preset changed the configuration: refresh what depends on it, once per job.
  const finishedId = job && isApplyJobTerminal(job) ? job.job_id : null
  useEffect(() => {
    if (finishedId === null || finishedRef.current === finishedId) return
    finishedRef.current = finishedId
    void queryClient.invalidateQueries({ queryKey: ['crowdsec-presets'] })
    void queryClient.invalidateQueries({ queryKey: ['crowdsec-files'] })
    void queryClient.invalidateQueries({ queryKey: ['security-status'] })
  }, [finishedId, queryClient])

  const follow = useCallback((started: CrowdsecApplyJob) => {
    queryClient.setQueryData([...APPLY_JOB_QUERY_KEY, 'job', started.job_id], started)
    setTracked({ id: started.job_id, dismissedId: null })
  }, [queryClient])

  const startMutation = useMutation({
    mutationFn: (request: ApplyJobRequest) => applyPresetAsync(request),
    onSuccess: follow,
    onError: (err: unknown) => {
      // The refusal names the job that holds the lock, so the row can show it.
      const busyJobId = isCrowdsecBusyError(err) ? getCrowdsecBusyJobId(err) : undefined
      if (busyJobId) setTracked((prev) => ({ ...prev, id: busyJobId }))
    },
  })

  const cancelMutation = useMutation({
    mutationFn: (id: string) => cancelApplyJob(id),
    onSuccess: follow,
    onError: (err: unknown) => {
      // Finished or unknown already: take the real state instead of guessing.
      if (isAxiosError(err) && jobId) {
        void queryClient.invalidateQueries({ queryKey: [...APPLY_JOB_QUERY_KEY, 'job', jobId] })
      }
    },
  })

  /** Hides the finished job's result (a new install replaces it anyway). */
  const dismiss = useCallback(() => {
    setTracked((prev) => ({ id: null, dismissedId: prev.id ?? jobId }))
  }, [jobId])

  return {
    job,
    isActive,
    isWaitingForServer,
    start: startMutation.mutateAsync,
    isStarting: startMutation.isPending,
    cancel: cancelMutation.mutate,
    isCancelling: cancelMutation.isPending,
    dismiss,
  }
}
