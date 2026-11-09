import { renderHook } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { useSeedOnChange } from '../useSeedOnChange'

describe('useSeedOnChange', () => {
  it('seeds once on mount when the source is truthy', () => {
    const onChange = vi.fn()
    const source = { host: 'smtp.local' }
    renderHook(() => useSeedOnChange(source, onChange))
    expect(onChange).toHaveBeenCalledTimes(1)
    expect(onChange).toHaveBeenCalledWith(source)
  })

  it('does not seed when the initial source is undefined', () => {
    const onChange = vi.fn()
    renderHook(() => useSeedOnChange(undefined, onChange))
    expect(onChange).not.toHaveBeenCalled()
  })

  it('does not seed on re-render while the source identity is unchanged', () => {
    const onChange = vi.fn()
    const source = { host: 'smtp.local' }
    const { rerender } = renderHook(({ value }) => useSeedOnChange(value, onChange), {
      initialProps: { value: source },
    })
    rerender({ value: source })
    rerender({ value: source })
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('seeds again when the source is replaced by a new object', () => {
    const onChange = vi.fn()
    const { rerender } = renderHook(({ value }) => useSeedOnChange(value, onChange), {
      initialProps: { value: { host: 'a' } as { host: string } | undefined },
    })
    rerender({ value: { host: 'b' } })
    expect(onChange).toHaveBeenCalledTimes(2)
    expect(onChange).toHaveBeenLastCalledWith({ host: 'b' })
  })

  it('skips a falsy source without clearing the last seeded value, then seeds when it returns', () => {
    const onChange = vi.fn()
    const first = { host: 'a' }
    const { rerender } = renderHook(({ value }) => useSeedOnChange(value, onChange), {
      initialProps: { value: first as { host: string } | undefined },
    })
    rerender({ value: undefined })
    expect(onChange).toHaveBeenCalledTimes(1)
    rerender({ value: first })
    expect(onChange).toHaveBeenCalledTimes(2)
    expect(onChange).toHaveBeenLastCalledWith(first)
  })
})
