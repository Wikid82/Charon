import { describe, it, expect } from 'vitest'

import { sanitizeKeptFileName } from '../keptFileNames'

describe('sanitizeKeptFileName', () => {
  it('keeps ordinary names unchanged', () => {
    expect(sanitizeKeptFileName('notifications/slack.yaml')).toBe('notifications/slack.yaml')
  })

  it('strips control, separator and direction-override characters', () => {
    expect(sanitizeKeptFileName('a\u0000b\nc\u007fd\u0085e f‮g⁦h.yaml')).toBe('abcdefgh.yaml')
  })

  it('caps long names', () => {
    const result = sanitizeKeptFileName('x'.repeat(200))
    expect(result).toBe(`${'x'.repeat(80)}…`)
  })

  it('does not truncate a name of exactly the maximum length', () => {
    expect(sanitizeKeptFileName('y'.repeat(80))).toBe('y'.repeat(80))
  })
})
