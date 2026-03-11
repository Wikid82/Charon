import { vi, describe, it, expect } from 'vitest'

// Mock the client module which is an axios instance wrapper
vi.mock('./client', () => ({
  default: {
    get: vi.fn(() => Promise.resolve({ data: { 'feature.cerberus.enabled': true } })),
    put: vi.fn(() => Promise.resolve({ data: { status: 'ok' } })),
  },
}))

import client from './client'
import { getFeatureFlags, updateFeatureFlags } from './featureFlags'

describe('featureFlags API', () => {
  it('fetches feature flags', async () => {
    const flags = await getFeatureFlags()
    expect(flags['feature.cerberus.enabled']).toBe(true)
    expect(vi.mocked(client.get)).toHaveBeenCalled()
  })

  it('updates feature flags', async () => {
    const resp = await updateFeatureFlags({ 'feature.cerberus.enabled': false })
    expect(resp).toEqual({ status: 'ok' })
    expect(vi.mocked(client.put)).toHaveBeenCalledWith('/feature-flags', { 'feature.cerberus.enabled': false })
  })
})
