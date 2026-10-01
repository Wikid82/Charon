import { describe, expect, it } from 'vitest'

import { formatBytes } from '../formatBytes'

describe('formatBytes', () => {
  it.each([
    [0, '0 B'],
    [-5, '0 B'],
    [Number.NaN, '0 B'],
    [512, '512 B'],
    [120_000_000, '120.0 MB'],
    [1_700_000_000, '1.7 GB'],
    [4_800_000_000, '4.8 GB'],
    [2_500_000_000_000, '2.5 TB'],
    [9_000_000_000_000_000, '9000.0 TB'],
  ])('formats %s as %s', (input, expected) => {
    expect(formatBytes(input)).toBe(expected)
  })
})
