import { render, screen, fireEvent } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'

import '@testing-library/jest-dom/vitest'
import { CustomColorPicker, hexToRgb, rgbToHex } from '../CustomColorPicker'

import type { CustomThemeColors } from '../../../context/ThemeContextValue'

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const map: Record<string, string> = {
        'appearance.colorPickerBgBase': 'Background',
        'appearance.colorPickerBgSubtle': 'Subtle Background',
        'appearance.colorPickerBgMuted': 'Muted Background',
        'appearance.colorPickerBgElevated': 'Elevated Surface',
        'appearance.colorPickerBorderDefault': 'Border',
        'appearance.colorPickerBorderStrong': 'Strong Border',
        'appearance.colorPickerTextPrimary': 'Primary Text',
        'appearance.colorPickerTextSecondary': 'Secondary Text',
        'appearance.colorPickerTextMuted': 'Muted Text',
        'appearance.colorPickerBrandPrimary': 'Accent Color',
        'appearance.colorPickerColorScheme': 'Base Scheme',
      }
      return map[key] ?? key
    },
  }),
}))

const defaultColors: CustomThemeColors = {
  bgBase: '15 23 42',
  bgSubtle: '30 41 59',
  bgMuted: '51 65 85',
  bgElevated: '30 41 59',
  borderDefault: '51 65 85',
  borderStrong: '71 85 105',
  textPrimary: '248 250 252',
  textSecondary: '203 213 225',
  textMuted: '148 163 184',
  brandPrimary: '59 130 246',
  colorScheme: 'dark',
}

describe('hexToRgb', () => {
  // CP-01
  it('CP-01: converts #3b82f6 to "59 130 246"', () => {
    expect(hexToRgb('#3b82f6')).toBe('59 130 246')
  })

  it('converts uppercase hex', () => {
    expect(hexToRgb('#3B82F6')).toBe('59 130 246')
  })

  it('converts black #000000', () => {
    expect(hexToRgb('#000000')).toBe('0 0 0')
  })

  it('converts white #ffffff', () => {
    expect(hexToRgb('#ffffff')).toBe('255 255 255')
  })
})

describe('rgbToHex', () => {
  // CP-02
  it('CP-02: converts "59 130 246" to "#3b82f6"', () => {
    expect(rgbToHex('59 130 246')).toBe('#3b82f6')
  })

  it('converts "0 0 0" to "#000000"', () => {
    expect(rgbToHex('0 0 0')).toBe('#000000')
  })

  it('converts "255 255 255" to "#ffffff"', () => {
    expect(rgbToHex('255 255 255')).toBe('#ffffff')
  })

  it('returns #000000 for malformed input', () => {
    expect(rgbToHex('not rgb')).toBe('#000000')
  })
})

describe('CustomColorPicker', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  // CP-04: Renders all 10 color inputs
  it('CP-04: renders 10 color inputs', () => {
    const onChange = vi.fn()
    render(<CustomColorPicker value={defaultColors} onChange={onChange} />)
    // Type="color" inputs are not accessible by role — use DOM query
    // eslint-disable-next-line testing-library/no-node-access
    const inputs = document.querySelectorAll('input[type="color"]')
    expect(inputs).toHaveLength(10)
  })

  it('CP-04: renders labels for all color fields', () => {
    const onChange = vi.fn()
    render(<CustomColorPicker value={defaultColors} onChange={onChange} />)
    expect(screen.getByLabelText('Background')).toBeInTheDocument()
    expect(screen.getByLabelText('Subtle Background')).toBeInTheDocument()
    expect(screen.getByLabelText('Muted Background')).toBeInTheDocument()
    expect(screen.getByLabelText('Elevated Surface')).toBeInTheDocument()
    expect(screen.getByLabelText('Border')).toBeInTheDocument()
    expect(screen.getByLabelText('Strong Border')).toBeInTheDocument()
    expect(screen.getByLabelText('Primary Text')).toBeInTheDocument()
    expect(screen.getByLabelText('Secondary Text')).toBeInTheDocument()
    expect(screen.getByLabelText('Muted Text')).toBeInTheDocument()
    expect(screen.getByLabelText('Accent Color')).toBeInTheDocument()
  })

  // CP-03: Changing bgBase color input triggers onChange with updated colors
  it('CP-03: changing bgBase triggers onChange with converted RGB value', () => {
    const onChange = vi.fn()
    render(<CustomColorPicker value={defaultColors} onChange={onChange} />)

    const bgBaseInput = screen.getByLabelText('Background')
    fireEvent.change(bgBaseInput, { target: { value: '#3b82f6' } })

    expect(onChange).toHaveBeenCalledOnce()
    const calledWith = onChange.mock.calls[0][0] as CustomThemeColors
    expect(calledWith.bgBase).toBe('59 130 246')
    // Other fields unchanged
    expect(calledWith.bgSubtle).toBe(defaultColors.bgSubtle)
    expect(calledWith.colorScheme).toBe('dark')
  })

  // CP-05: Color scheme toggle switches colorScheme between dark/light
  it('CP-05: changing color scheme select to "light" triggers onChange with colorScheme light', () => {
    const onChange = vi.fn()
    render(<CustomColorPicker value={defaultColors} onChange={onChange} />)

    const schemeSelect = screen.getByLabelText('Base Scheme')
    fireEvent.change(schemeSelect, { target: { value: 'light' } })

    expect(onChange).toHaveBeenCalledOnce()
    const calledWith = onChange.mock.calls[0][0] as CustomThemeColors
    expect(calledWith.colorScheme).toBe('light')
  })

  it('CP-05: changing color scheme select back to "dark" triggers onChange with colorScheme dark', () => {
    const onChange = vi.fn()
    const lightColors = { ...defaultColors, colorScheme: 'light' as const }
    render(<CustomColorPicker value={lightColors} onChange={onChange} />)

    const schemeSelect = screen.getByLabelText('Base Scheme')
    fireEvent.change(schemeSelect, { target: { value: 'dark' } })

    expect(onChange).toHaveBeenCalledOnce()
    const calledWith = onChange.mock.calls[0][0] as CustomThemeColors
    expect(calledWith.colorScheme).toBe('dark')
  })

  it('displays current color values as hex in the inputs', () => {
    const onChange = vi.fn()
    render(<CustomColorPicker value={defaultColors} onChange={onChange} />)

    const bgBaseInput = screen.getByLabelText('Background') as HTMLInputElement
    // '15 23 42' → '#0f172a'
    expect(bgBaseInput.value).toBe('#0f172a')
  })

  it('renders color scheme select with current value', () => {
    const onChange = vi.fn()
    render(<CustomColorPicker value={defaultColors} onChange={onChange} />)

    const schemeSelect = screen.getByLabelText('Base Scheme') as HTMLSelectElement
    expect(schemeSelect.value).toBe('dark')
  })
})
