import { describe, it, expect } from 'vitest'
import { calculatePasswordStrength } from '../passwordStrength'

describe('calculatePasswordStrength', () => {
  it('returns score 0 for empty password', () => {
    const result = calculatePasswordStrength('')
    expect(result.score).toBe(0)
    expect(result.label).toBe('Empty')
  })

  it('returns low score for short password', () => {
    const result = calculatePasswordStrength('short')
    expect(result.score).toBeLessThan(2)
  })

  it('returns higher score for longer password', () => {
    const result = calculatePasswordStrength('longerpassword')
    expect(result.score).toBeGreaterThanOrEqual(2)
  })

  it('rewards complexity (numbers, symbols, uppercase)', () => {
    const simple = calculatePasswordStrength('password123')
    const complex = calculatePasswordStrength('Password123!')

    expect(complex.score).toBeGreaterThan(simple.score)
  })

  it('returns max score for strong password', () => {
    const result = calculatePasswordStrength('CorrectHorseBatteryStaple1!')
    expect(result.score).toBe(4)
    expect(result.label).toBe('Strong')
  })

  it('provides feedback for weak passwords', () => {
    const result = calculatePasswordStrength('123456')
    expect(result.feedback).toBeDefined()
    // The feedback is an array of strings
    expect(result.feedback.length).toBeGreaterThan(0)
  })
})
