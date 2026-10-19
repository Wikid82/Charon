import { render, screen } from '@testing-library/react'
import { describe, it, expect, vi } from 'vitest'

import { ListErrorNote } from '../crowdsec/ListErrorNote'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => `[${key}] ${fallback ?? ''}`.trim(),
  }),
}))

describe('ListErrorNote', () => {
  it.each(['cscli_timeout', 'cscli_output_too_large', 'cscli_unavailable'] as const)(
    'shows the translated text for %s',
    (code) => {
      render(<ListErrorNote error="english text" error_code={code} />)
      expect(screen.getByRole('note')).toHaveTextContent(`[security.crowdsec.dashboard.listErrors.${code}]`)
    },
  )

  it('falls back to the English error text for an unknown code', () => {
    render(<ListErrorNote error="something odd" error_code={'brand_new' as never} />)
    expect(screen.getByRole('note')).toHaveTextContent('something odd')
    expect(screen.getByRole('note')).not.toHaveTextContent('listErrors')
  })

  it('falls back to the English error text when there is no code', () => {
    render(<ListErrorNote error="plain failure" />)
    expect(screen.getByRole('note')).toHaveTextContent('plain failure')
  })

  it('renders nothing when the response has no error', () => {
    const { container } = render(<ListErrorNote />)
    expect(container).toBeEmptyDOMElement()
  })
})
