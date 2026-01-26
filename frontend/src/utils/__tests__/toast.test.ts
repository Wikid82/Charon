import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { toast, toastCallbacks } from '../toast'

describe('toast util', () => {
  beforeEach(() => {
    // Ensure callbacks set is empty before each test
    toastCallbacks.clear()
  })

  afterEach(() => {
    toastCallbacks.clear()
  })

  it('calls registered callbacks for each toast type', () => {
    const mock = vi.fn()
    toastCallbacks.add(mock)

    toast.success('ok')
    toast.error('bad')
    toast.info('info')
    toast.warning('warn')

    expect(mock).toHaveBeenCalledTimes(4)
    expect(mock.mock.calls[0][0]).toMatchObject({ message: 'ok', type: 'success' })
    expect(mock.mock.calls[1][0]).toMatchObject({ message: 'bad', type: 'error' })
    expect(mock.mock.calls[2][0]).toMatchObject({ message: 'info', type: 'info' })
    expect(mock.mock.calls[3][0]).toMatchObject({ message: 'warn', type: 'warning' })
  })

  it('provides incrementing ids', () => {
    const mock = vi.fn()
    toastCallbacks.add(mock)
    // send multiple messages
    toast.success('one')
    toast.success('two')
    const firstId = mock.mock.calls[0][0].id
    const secondId = mock.mock.calls[1][0].id
    expect(secondId).toBeGreaterThan(firstId)
  })
})
