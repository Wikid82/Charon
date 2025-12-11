import { describe, it, expect } from 'vitest'

describe('Test setup file checks', () => {
  it('sets the React act environment flag', () => {
    expect(globalThis.IS_REACT_ACT_ENVIRONMENT).toBe(true)
  })

  it('stubs window.matchMedia with expected interface', () => {
    const mq = window.matchMedia('(min-width: 100px)')
    expect(mq.matches).toBe(false)
    expect(typeof mq.addListener).toBe('function')
    expect(typeof mq.removeListener).toBe('function')
    expect(typeof mq.addEventListener).toBe('function')
    expect(typeof mq.removeEventListener).toBe('function')
  })
})
