import { vi, describe, it, expect } from 'vitest'

// Mock the client module which is an axios instance wrapper
vi.mock('./client', () => ({
  default: {
    get: vi.fn(() => Promise.resolve({ data: { 'feature.global.enabled': true } })),
    put: vi.fn(() => Promise.resolve({ data: { status: 'ok' } })),
  },
}))

import { getFeatureFlags, updateFeatureFlags } from './featureFlags'
import client from './client'

describe('featureFlags API', () => {
  it('fetches feature flags', async () => {
    const flags = await getFeatureFlags()
    expect(flags['feature.global.enabled']).toBe(true)
    expect((client.get as any)).toHaveBeenCalled()
  })

  it('updates feature flags', async () => {
    const resp = await updateFeatureFlags({ 'feature.global.enabled': false })
    expect(resp).toEqual({ status: 'ok' })
    expect((client.put as any)).toHaveBeenCalledWith('/feature-flags', { 'feature.global.enabled': false })
  })
})
