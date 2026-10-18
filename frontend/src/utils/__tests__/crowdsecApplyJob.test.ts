import { describe, expect, it } from 'vitest'

import { isApplyJobTerminal } from '../crowdsecApplyJob'

describe('isApplyJobTerminal', () => {
  it('is false while the job is queued, running or verifying', () => {
    for (const state of ['queued', 'running', 'verifying'] as const) {
      expect(isApplyJobTerminal({ state })).toBe(false)
    }
  })

  it('is true for every ended state', () => {
    for (const state of ['succeeded', 'failed', 'timed_out', 'cancelled'] as const) {
      expect(isApplyJobTerminal({ state })).toBe(true)
    }
  })
})
