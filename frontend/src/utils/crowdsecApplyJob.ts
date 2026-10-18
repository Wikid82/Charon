import type { CrowdsecApplyJob, CrowdsecApplyJobState } from '../api/presets'

const TERMINAL_JOB_STATES: readonly CrowdsecApplyJobState[] = ['succeeded', 'failed', 'timed_out', 'cancelled']

/** True once the install job has finished, whatever the outcome. */
export function isApplyJobTerminal(job: Pick<CrowdsecApplyJob, 'state'>): boolean {
  return TERMINAL_JOB_STATES.includes(job.state)
}
