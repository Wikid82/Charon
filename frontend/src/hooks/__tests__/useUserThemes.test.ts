import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor, act } from '@testing-library/react'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { createElement } from 'react'

import { useUserThemes } from '../useUserThemes'
import * as themesApi from '../../api/themes'
import type { UserThemeDTO } from '../../api/themes'
import type { CustomThemeColors } from '../../context/ThemeContextValue'

vi.mock('../../api/themes', () => ({
  listUserThemes: vi.fn(),
  createUserTheme: vi.fn(),
  updateUserTheme: vi.fn(),
  deleteUserTheme: vi.fn(),
  parseUserThemeDTO: vi.fn((dto: UserThemeDTO) => ({
    id: dto.id,
    name: dto.name,
    colors: JSON.parse(dto.colors) as CustomThemeColors,
    created_at: dto.created_at,
    updated_at: dto.updated_at,
  })),
}))

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

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return ({ children }: { children: React.ReactNode }) =>
    createElement(QueryClientProvider, { client: queryClient }, children)
}

describe('useUserThemes', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('fetches themes and parses them via parseUserThemeDTO', async () => {
    vi.mocked(themesApi.listUserThemes).mockResolvedValue([sampleDTO])

    const { result } = renderHook(() => useUserThemes(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(themesApi.listUserThemes).toHaveBeenCalled()
    // parseUserThemeDTO is called with (dto, index, array) — check the first argument only
    expect(themesApi.parseUserThemeDTO).toHaveBeenCalledWith(sampleDTO, 0, [sampleDTO])
    expect(result.current.userThemes).toHaveLength(1)
    expect(result.current.userThemes[0].id).toBe('uuid-1')
    expect(result.current.userThemes[0].name).toBe('My Dark Theme')
    expect(typeof result.current.userThemes[0].colors).toBe('object')
  })

  it('returns empty array when no themes exist', async () => {
    vi.mocked(themesApi.listUserThemes).mockResolvedValue([])

    const { result } = renderHook(() => useUserThemes(), { wrapper: createWrapper() })

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })

    expect(result.current.userThemes).toEqual([])
  })

  it('createTheme mutation invalidates user-themes query', async () => {
    vi.mocked(themesApi.listUserThemes).mockResolvedValue([])
    vi.mocked(themesApi.createUserTheme).mockResolvedValue(sampleDTO)

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    const { result } = renderHook(() => useUserThemes(), {
      wrapper: ({ children }) => createElement(QueryClientProvider, { client: queryClient }, children),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    await act(async () => {
      await result.current.createTheme('My Dark Theme', sampleColors)
    })

    // TanStack Query v5 passes context as a second argument to mutationFn — check first arg
    const createCall = vi.mocked(themesApi.createUserTheme).mock.calls[0]
    expect(createCall[0]).toEqual({ name: 'My Dark Theme', colors: sampleColors })
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['user-themes'] })
  })

  it('deleteTheme mutation invalidates user-themes query', async () => {
    vi.mocked(themesApi.listUserThemes).mockResolvedValue([sampleDTO])
    vi.mocked(themesApi.deleteUserTheme).mockResolvedValue(undefined)

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    const { result } = renderHook(() => useUserThemes(), {
      wrapper: ({ children }) => createElement(QueryClientProvider, { client: queryClient }, children),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    await act(async () => {
      await result.current.deleteTheme('uuid-1')
    })

    // TanStack Query v5 passes context as a second argument to mutationFn — check first arg
    const deleteCall = vi.mocked(themesApi.deleteUserTheme).mock.calls[0]
    expect(deleteCall[0]).toBe('uuid-1')
    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['user-themes'] })
  })

  it('updateTheme mutation invalidates user-themes query', async () => {
    vi.mocked(themesApi.listUserThemes).mockResolvedValue([sampleDTO])
    vi.mocked(themesApi.updateUserTheme).mockResolvedValue(sampleDTO)

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    const { result } = renderHook(() => useUserThemes(), {
      wrapper: ({ children }) => createElement(QueryClientProvider, { client: queryClient }, children),
    })

    await waitFor(() => expect(result.current.isLoading).toBe(false))

    await act(async () => {
      await result.current.updateTheme('uuid-1', { name: 'Updated Name' })
    })

    expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['user-themes'] })
  })

  it('exposes isCreating, isUpdating, isDeleting flags', () => {
    vi.mocked(themesApi.listUserThemes).mockResolvedValue([])

    const { result } = renderHook(() => useUserThemes(), { wrapper: createWrapper() })

    expect(typeof result.current.isCreating).toBe('boolean')
    expect(typeof result.current.isUpdating).toBe('boolean')
    expect(typeof result.current.isDeleting).toBe('boolean')
  })
})
