import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { LanguageSelector } from '../LanguageSelector'
import { LanguageProvider } from '../../context/LanguageContext'

// Mock i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: {
      changeLanguage: vi.fn(),
      language: 'en',
    },
  }),
}))

describe('LanguageSelector', () => {
  const renderWithProvider = () => {
    return render(
      <LanguageProvider>
        <LanguageSelector />
      </LanguageProvider>
    )
  }

  it('renders language selector with all options', () => {
    renderWithProvider()

    const select = screen.getByRole('combobox')
    expect(select).toBeInTheDocument()

    // Check that all language options are available
    const options = screen.getAllByRole('option')
    expect(options).toHaveLength(5)
    expect(options[0]).toHaveTextContent('English')
    expect(options[1]).toHaveTextContent('Español')
    expect(options[2]).toHaveTextContent('Français')
    expect(options[3]).toHaveTextContent('Deutsch')
    expect(options[4]).toHaveTextContent('中文')
  })

  it('displays globe icon', () => {
    const { container } = renderWithProvider()
    const svgElement = container.querySelector('svg')
    expect(svgElement).toBeInTheDocument()
  })

  it('changes language when option is selected', () => {
    renderWithProvider()

    const select = screen.getByRole('combobox') as HTMLSelectElement
    expect(select.value).toBe('en')

    fireEvent.change(select, { target: { value: 'es' } })
    expect(select.value).toBe('es')

    fireEvent.change(select, { target: { value: 'fr' } })
    expect(select.value).toBe('fr')
  })
})
