import { describe, expect, it } from 'vitest'

import { usedTranslationKeys } from './localeKeys'

describe('usedTranslationKeys', () => {
  it('returns each distinct static key passed to t()', () => {
    const source = "t('a.one'); t( 'b.two' ); t('a.one'); t(`dynamic.${x}`); t(someVar)"

    expect(usedTranslationKeys(source)).toEqual(['a.one', 'b.two'])
  })

  it('returns an empty list when the source has no static keys', () => {
    expect(usedTranslationKeys('const value = 1')).toEqual([])
  })
})
