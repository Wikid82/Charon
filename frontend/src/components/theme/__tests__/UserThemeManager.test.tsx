import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'

import { UserThemeManager } from '../UserThemeManager'
import type { UserTheme, CustomThemeColors } from '../../../context/ThemeContextValue'

// Mock the useUserThemes hook
vi.mock('../../../hooks/useUserThemes', () => ({
  useUserThemes: vi.fn(),
}))

import { useUserThemes } from '../../../hooks/useUserThemes'

const mockUseUserThemes = vi.mocked(useUserThemes)

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

const sampleTheme: UserTheme = {
  id: 'uuid-1',
  name: 'My Dark Theme',
  colors: sampleColors,
  created_at: '2026-06-21T10:00:00Z',
  updated_at: '2026-06-21T10:00:00Z',
}

const sampleTheme2: UserTheme = {
  id: 'uuid-2',
  name: 'Ocean Blue',
  colors: { ...sampleColors, bgBase: '5 15 35', brandPrimary: '20 100 200' },
  created_at: '2026-06-21T11:00:00Z',
  updated_at: '2026-06-21T11:00:00Z',
}

function setupMock(overrides: Partial<ReturnType<typeof useUserThemes>> = {}) {
  mockUseUserThemes.mockReturnValue({
    userThemes: [],
    isLoading: false,
    error: null,
    createTheme: vi.fn().mockResolvedValue(undefined),
    updateTheme: vi.fn().mockResolvedValue(undefined),
    deleteTheme: vi.fn().mockResolvedValue(undefined),
    isCreating: false,
    isUpdating: false,
    isDeleting: false,
    ...overrides,
  })
}

describe('UserThemeManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    setupMock()
  })

  // UTM-01: Empty state
  it('UTM-01: shows empty state message when no themes exist', () => {
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)
    expect(screen.getByText('No saved themes yet. Create one to get started.')).toBeInTheDocument()
  })

  // UTM-02: Theme list renders cards
  it('UTM-02: renders theme cards with names', () => {
    setupMock({ userThemes: [sampleTheme, sampleTheme2] })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    expect(screen.getByText('My Dark Theme')).toBeInTheDocument()
    expect(screen.getByText('Ocean Blue')).toBeInTheDocument()
  })

  // UTM-03: Activate button calls onActivate
  it('UTM-03: Activate button calls onActivate with the correct theme', async () => {
    const onActivate = vi.fn()
    setupMock({ userThemes: [sampleTheme] })
    render(<UserThemeManager activeThemeId="dark" onActivate={onActivate} />)

    const activateButton = screen.getByRole('button', { name: /activate my dark theme/i })
    await userEvent.click(activateButton)

    expect(onActivate).toHaveBeenCalledWith(sampleTheme)
  })

  // UTM-04: Active theme shows checkmark instead of activate button
  it('UTM-04: active theme shows checkmark instead of Activate button', () => {
    setupMock({ userThemes: [sampleTheme] })
    render(<UserThemeManager activeThemeId="user:uuid-1" onActivate={vi.fn()} />)

    expect(screen.queryByRole('button', { name: /activate my dark theme/i })).not.toBeInTheDocument()
    expect(screen.getByText('Active')).toBeInTheDocument()
  })

  // UTM-05: Create dialog opens
  it('UTM-05: Create New Theme button opens dialog', async () => {
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    const createButton = screen.getByRole('button', { name: /create new theme/i })
    await userEvent.click(createButton)

    await waitFor(() => {
      // Dialog title should appear
      expect(screen.getAllByText('Create New Theme').length).toBeGreaterThan(0)
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })
  })

  // UTM-06: Create dialog has name input
  it('UTM-06: create dialog contains name input and cancel button', async () => {
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    const createButton = screen.getByRole('button', { name: /create new theme/i })
    await userEvent.click(createButton)

    await waitFor(() => {
      expect(screen.getByLabelText('Theme Name')).toBeInTheDocument()
    })
    expect(screen.getByPlaceholderText('e.g. My Dark Theme')).toBeInTheDocument()
  })

  // UTM-07: Edit dialog opens with existing values
  it('UTM-07: Edit button opens dialog with theme name pre-filled', async () => {
    setupMock({ userThemes: [sampleTheme] })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    const editButton = screen.getByRole('button', { name: /edit theme my dark theme/i })
    await userEvent.click(editButton)

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toBeInTheDocument()
    })

    const nameInput = screen.getByLabelText('Theme Name') as HTMLInputElement
    expect(nameInput.value).toBe('My Dark Theme')
  })

  // UTM-08: Delete confirm dialog appears
  it('UTM-08: Delete button opens confirm dialog', async () => {
    setupMock({ userThemes: [sampleTheme] })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    const deleteButton = screen.getByRole('button', { name: /delete theme my dark theme/i })
    await userEvent.click(deleteButton)

    await waitFor(() => {
      expect(screen.getByText('Delete this theme? This cannot be undone.')).toBeInTheDocument()
    })
  })

  // UTM-09: Confirm delete calls deleteTheme
  it('UTM-09: confirming delete calls deleteTheme with correct id', async () => {
    const deleteTheme = vi.fn().mockResolvedValue(undefined)
    setupMock({ userThemes: [sampleTheme], deleteTheme })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    const deleteButton = screen.getByRole('button', { name: /delete theme my dark theme/i })
    await userEvent.click(deleteButton)

    await waitFor(() => {
      expect(screen.getByText('Delete this theme? This cannot be undone.')).toBeInTheDocument()
    })

    // Click the confirm delete button in the alert dialog
    const confirmButtons = screen.getAllByRole('button', { name: /delete theme/i })
    // The confirm button in the alertdialog
    const confirmButton = confirmButtons.find(btn => btn.closest('[role="alertdialog"]'))
    if (confirmButton) {
      await userEvent.click(confirmButton)
    }

    await waitFor(() => {
      expect(deleteTheme).toHaveBeenCalledWith('uuid-1')
    })
  })

  // UTM-10: Loading state
  it('UTM-10: shows loading state while fetching', () => {
    setupMock({ isLoading: true })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  // UTM-11: Color swatch bar renders for each theme
  it('UTM-11: renders color swatches for theme cards', () => {
    setupMock({ userThemes: [sampleTheme] })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    // Each card should have 5 swatch spans (hidden from accessibility tree with aria-hidden)
    const swatches = document.querySelectorAll('[aria-hidden="true"]')
    expect(swatches.length).toBeGreaterThanOrEqual(5)
  })

  // UTM-12: Typing in create dialog name input changes the value
  it('UTM-12: typing in create dialog name input updates the field', async () => {
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: /create new theme/i }))
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())

    const nameInput = screen.getByLabelText('Theme Name') as HTMLInputElement
    await userEvent.type(nameInput, 'Midnight')
    expect(nameInput.value).toBe('Midnight')
  })

  // UTM-13: Cancel button in create dialog closes it without calling createTheme
  it('UTM-13: cancel in create dialog closes dialog without creating theme', async () => {
    const createTheme = vi.fn()
    setupMock({ createTheme })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: /create new theme/i }))
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())

    await userEvent.click(screen.getByRole('button', { name: /^cancel$/i }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(createTheme).not.toHaveBeenCalled()
  })

  // UTM-14: Saving the create form calls createTheme and closes the dialog
  it('UTM-14: submitting create dialog calls createTheme with the entered name', async () => {
    const createTheme = vi.fn().mockResolvedValue(undefined)
    setupMock({ createTheme })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: /create new theme/i }))
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())

    await userEvent.type(screen.getByLabelText('Theme Name'), 'Midnight Blue')
    await userEvent.click(screen.getByRole('button', { name: /save theme/i }))

    await waitFor(() => expect(createTheme).toHaveBeenCalledWith('Midnight Blue', expect.any(Object)))
  })

  // UTM-15: Edit dialog: change name + cancel without saving
  it('UTM-15: cancel in edit dialog closes it without calling updateTheme', async () => {
    const updateTheme = vi.fn()
    setupMock({ userThemes: [sampleTheme], updateTheme })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: /edit theme my dark theme/i }))
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())

    const nameInput = screen.getByLabelText('Theme Name') as HTMLInputElement
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'Renamed')
    expect(nameInput.value).toBe('Renamed')

    await userEvent.click(screen.getByRole('button', { name: /^cancel$/i }))

    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(updateTheme).not.toHaveBeenCalled()
  })

  // UTM-16: Saving the edit dialog calls updateTheme with the new name
  it('UTM-16: submitting edit dialog calls updateTheme with updated name', async () => {
    const updateTheme = vi.fn().mockResolvedValue(undefined)
    setupMock({ userThemes: [sampleTheme], updateTheme })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: /edit theme my dark theme/i }))
    await waitFor(() => expect(screen.getByRole('dialog')).toBeInTheDocument())

    const nameInput = screen.getByLabelText('Theme Name') as HTMLInputElement
    await userEvent.clear(nameInput)
    await userEvent.type(nameInput, 'Updated Name')

    await userEvent.click(screen.getByRole('button', { name: /save theme/i }))

    await waitFor(() =>
      expect(updateTheme).toHaveBeenCalledWith('uuid-1', expect.objectContaining({ name: 'Updated Name' }))
    )
  })

  // UTM-17: Cancel button in delete confirm dialog closes it without calling deleteTheme
  it('UTM-17: cancel in delete confirm dialog does not call deleteTheme', async () => {
    const deleteTheme = vi.fn()
    setupMock({ userThemes: [sampleTheme], deleteTheme })
    render(<UserThemeManager activeThemeId="dark" onActivate={vi.fn()} />)

    await userEvent.click(screen.getByRole('button', { name: /delete theme my dark theme/i }))
    await waitFor(() => expect(screen.getByText('Delete this theme? This cannot be undone.')).toBeInTheDocument())

    const cancelBtn = screen.getAllByRole('button', { name: /^cancel$/i })[0]
    await userEvent.click(cancelBtn)

    await waitFor(() =>
      expect(screen.queryByText('Delete this theme? This cannot be undone.')).not.toBeInTheDocument()
    )
    expect(deleteTheme).not.toHaveBeenCalled()
  })
})
