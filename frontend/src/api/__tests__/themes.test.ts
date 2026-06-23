import { describe, it, expect, vi, beforeEach } from 'vitest'

import client from '../client'
import {
  listUserThemes,
  createUserTheme,
  updateUserTheme,
  deleteUserTheme,
  parseUserThemeDTO,
  type UserThemeDTO,
} from '../themes'

import type { CustomThemeColors } from '../../context/ThemeContextValue'

vi.mock('../client')

const sampleColors: CustomThemeColors = {
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

const sampleDTO: UserThemeDTO = {
  id: 'uuid-1',
  name: 'My Dark Theme',
  colors: JSON.stringify(sampleColors),
  created_at: '2026-06-21T10:00:00Z',
  updated_at: '2026-06-21T10:00:00Z',
}

describe('themes API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('parseUserThemeDTO', () => {
    it('parses colors from JSON string to CustomThemeColors object', () => {
      const result = parseUserThemeDTO(sampleDTO)
      expect(result.id).toBe('uuid-1')
      expect(result.name).toBe('My Dark Theme')
      expect(result.colors).toEqual(sampleColors)
      expect(result.created_at).toBe('2026-06-21T10:00:00Z')
      expect(result.updated_at).toBe('2026-06-21T10:00:00Z')
    })

    it('returns a typed UserTheme with colors as object (not string)', () => {
      const result = parseUserThemeDTO(sampleDTO)
      expect(typeof result.colors).toBe('object')
      expect(result.colors.bgBase).toBe('15 23 42')
    })
  })

  describe('listUserThemes', () => {
    it('calls GET /themes and returns data', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [sampleDTO] })

      const result = await listUserThemes()

      expect(client.get).toHaveBeenCalledWith('/themes')
      expect(result).toEqual([sampleDTO])
    })

    it('returns empty array when no themes exist', async () => {
      vi.mocked(client.get).mockResolvedValue({ data: [] })

      const result = await listUserThemes()
      expect(result).toEqual([])
    })
  })

  describe('createUserTheme', () => {
    it('serializes colors to JSON string before sending', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: sampleDTO })

      await createUserTheme({ name: 'My Dark Theme', colors: sampleColors })

      expect(client.post).toHaveBeenCalledWith('/themes', {
        name: 'My Dark Theme',
        colors: JSON.stringify(sampleColors),
      })
    })

    it('returns the created DTO from response', async () => {
      vi.mocked(client.post).mockResolvedValue({ data: sampleDTO })

      const result = await createUserTheme({ name: 'My Dark Theme', colors: sampleColors })
      expect(result).toEqual(sampleDTO)
    })
  })

  describe('updateUserTheme', () => {
    it('only sends defined fields in the body', async () => {
      vi.mocked(client.put).mockResolvedValue({ data: sampleDTO })

      await updateUserTheme('uuid-1', { name: 'Updated Name' })

      expect(client.put).toHaveBeenCalledWith('/themes/uuid-1', {
        name: 'Updated Name',
      })
      const callBody = vi.mocked(client.put).mock.calls[0][1] as Record<string, unknown>
      expect('colors' in callBody).toBe(false)
    })

    it('serializes colors when provided', async () => {
      vi.mocked(client.put).mockResolvedValue({ data: sampleDTO })

      await updateUserTheme('uuid-1', { colors: sampleColors })

      const callBody = vi.mocked(client.put).mock.calls[0][1] as Record<string, unknown>
      expect(callBody.colors).toBe(JSON.stringify(sampleColors))
      expect('name' in callBody).toBe(false)
    })

    it('sends both name and colors when both are defined', async () => {
      vi.mocked(client.put).mockResolvedValue({ data: sampleDTO })

      await updateUserTheme('uuid-1', { name: 'New Name', colors: sampleColors })

      expect(client.put).toHaveBeenCalledWith('/themes/uuid-1', {
        name: 'New Name',
        colors: JSON.stringify(sampleColors),
      })
    })
  })

  describe('deleteUserTheme', () => {
    it('calls DELETE /themes/:id', async () => {
      vi.mocked(client.delete).mockResolvedValue({ data: {} })

      await deleteUserTheme('uuid-1')

      expect(client.delete).toHaveBeenCalledWith('/themes/uuid-1')
    })
  })
})
