import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Check, Pencil, Trash2 } from 'lucide-react'

import { useUserThemes } from '../../hooks/useUserThemes'
import { CustomColorPicker } from './CustomColorPicker'
import { Button } from '../ui/Button'
import { Input } from '../ui/Input'
import { Label } from '../ui/Label'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from '../ui/Dialog'

import type { ThemeId, UserTheme, CustomThemeColors } from '../../context/ThemeContextValue'

// Default colors based on the dark theme
const DARK_THEME_DEFAULTS: CustomThemeColors = {
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

const SWATCH_FIELDS: Array<keyof Omit<CustomThemeColors, 'colorScheme'>> = [
  'bgBase', 'bgSubtle', 'brandPrimary', 'textPrimary', 'borderDefault',
]

export interface UserThemeManagerProps {
  activeThemeId: ThemeId
  onActivate: (theme: UserTheme) => void
}

export function UserThemeManager({ activeThemeId, onActivate }: UserThemeManagerProps) {
  const { t } = useTranslation()
  const { userThemes, isLoading, createTheme, updateTheme, deleteTheme, isCreating, isUpdating, isDeleting } = useUserThemes()

  const [createDialogOpen, setCreateDialogOpen] = useState(false)
  const [editingThemeId, setEditingThemeId] = useState<string | null>(null)
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null)

  // Create dialog state
  const [createName, setCreateName] = useState('')
  const [createColors, setCreateColors] = useState<CustomThemeColors>(DARK_THEME_DEFAULTS)

  // Edit dialog state
  const [editName, setEditName] = useState('')
  const [editColors, setEditColors] = useState<CustomThemeColors>(DARK_THEME_DEFAULTS)

  const editingTheme = editingThemeId ? userThemes.find(t => t.id === editingThemeId) ?? null : null

  const handleOpenCreate = () => {
    setCreateName('')
    setCreateColors(DARK_THEME_DEFAULTS)
    setCreateDialogOpen(true)
  }

  const handleCreate = async () => {
    if (!createName.trim()) return
    await createTheme(createName.trim(), createColors)
    setCreateDialogOpen(false)
  }

  const handleOpenEdit = (theme: UserTheme) => {
    setEditingThemeId(theme.id)
    setEditName(theme.name)
    setEditColors(theme.colors)
  }

  const handleUpdate = async () => {
    if (!editingThemeId || !editName.trim()) return
    await updateTheme(editingThemeId, { name: editName.trim(), colors: editColors })
    setEditingThemeId(null)
  }

  const handleDelete = async () => {
    if (!confirmDeleteId) return
    await deleteTheme(confirmDeleteId)
    setConfirmDeleteId(null)
  }

  if (isLoading) {
    return <div className="text-sm text-content-muted">{t('common.loading')}</div>
  }

  return (
    <div className="space-y-4">
      {/* Header row */}
      <div className="flex items-center justify-between">
        <span className="sr-only">{t('appearance.userThemes')}</span>
        <Button
          variant="primary"
          size="sm"
          onClick={handleOpenCreate}
          aria-label={t('appearance.createNewTheme')}
        >
          + {t('appearance.createNewTheme')}
        </Button>
      </div>

      {/* Empty state */}
      {userThemes.length === 0 && (
        <p className="text-sm text-content-muted">{t('appearance.noUserThemes')}</p>
      )}

      {/* Theme cards grid */}
      {userThemes.length > 0 && (
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
          {userThemes.map((theme) => {
            const themeId = `user:${theme.id}` as ThemeId
            const isActive = activeThemeId === themeId

            return (
              <div
                key={theme.id}
                className={`rounded-lg border p-3 space-y-2 transition-colors ${
                  isActive ? 'border-brand-500 bg-brand-500/5' : 'border-border bg-surface-elevated'
                }`}
              >
                {/* Theme name */}
                <p className="text-sm font-medium text-content-primary truncate" title={theme.name}>
                  {theme.name}
                </p>

                {/* Color swatch bar */}
                <div className="flex h-4 rounded overflow-hidden">
                  {SWATCH_FIELDS.map((field) => (
                    <span
                      key={field}
                      className="flex-1"
                      style={{ background: `rgb(${theme.colors[field]})` }}
                      aria-hidden="true"
                    />
                  ))}
                </div>

                {/* Actions */}
                <div className="flex items-center gap-1">
                  {isActive ? (
                    <span
                      className="flex items-center gap-1 text-xs text-brand-500 font-medium"
                      aria-label="Active theme"
                    >
                      <Check className="h-3 w-3" />
                      Active
                    </span>
                  ) : (
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => onActivate(theme)}
                      aria-label={`${t('appearance.activateTheme')} ${theme.name}`}
                      className="text-xs h-7 px-2"
                    >
                      {t('appearance.activateTheme')}
                    </Button>
                  )}
                  <div className="ml-auto flex gap-1">
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => handleOpenEdit(theme)}
                      aria-label={`${t('appearance.editTheme')} ${theme.name}`}
                      className="h-7 w-7 p-0"
                    >
                      <Pencil className="h-3 w-3" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="sm"
                      onClick={() => setConfirmDeleteId(theme.id)}
                      aria-label={`${t('appearance.deleteTheme')} ${theme.name}`}
                      className="h-7 w-7 p-0 text-error hover:text-error hover:bg-error-muted"
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                  </div>
                </div>
              </div>
            )
          })}
        </div>
      )}

      {/* Create New Theme Dialog */}
      <Dialog open={createDialogOpen} onOpenChange={setCreateDialogOpen}>
        <DialogContent
          role="dialog"
          aria-modal="true"
          aria-labelledby="create-theme-dialog-title"
        >
          <DialogHeader>
            <DialogTitle id="create-theme-dialog-title">
              {t('appearance.createNewTheme')}
            </DialogTitle>
          </DialogHeader>
          <div className="px-6 space-y-4">
            <div className="space-y-1">
              <Label htmlFor="create-theme-name">{t('appearance.themeNameLabel')}</Label>
              <Input
                id="create-theme-name"
                type="text"
                value={createName}
                onChange={(e) => setCreateName(e.target.value)}
                placeholder={t('appearance.themeNamePlaceholder')}
              />
            </div>
            <CustomColorPicker value={createColors} onChange={setCreateColors} />
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setCreateDialogOpen(false)}
            >
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => { void handleCreate() }}
              disabled={!createName.trim() || isCreating}
            >
              {t('appearance.saveTheme')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Edit Theme Dialog */}
      <Dialog open={editingThemeId !== null} onOpenChange={(open) => { if (!open) setEditingThemeId(null) }}>
        <DialogContent
          role="dialog"
          aria-modal="true"
          aria-labelledby="edit-theme-dialog-title"
        >
          <DialogHeader>
            <DialogTitle id="edit-theme-dialog-title">
              {t('appearance.editTheme')}
            </DialogTitle>
          </DialogHeader>
          <div className="px-6 space-y-4">
            <div className="space-y-1">
              <Label htmlFor="edit-theme-name">{t('appearance.themeNameLabel')}</Label>
              <Input
                id="edit-theme-name"
                type="text"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                placeholder={t('appearance.themeNamePlaceholder')}
              />
            </div>
            {editingTheme && (
              <CustomColorPicker value={editColors} onChange={setEditColors} />
            )}
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setEditingThemeId(null)}
            >
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => { void handleUpdate() }}
              disabled={!editName.trim() || isUpdating}
            >
              {t('appearance.saveTheme')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Delete Confirm Dialog */}
      <Dialog open={confirmDeleteId !== null} onOpenChange={(open) => { if (!open) setConfirmDeleteId(null) }}>
        <DialogContent
          role="alertdialog"
          aria-modal="true"
          aria-labelledby="delete-theme-dialog-title"
        >
          <DialogHeader>
            <DialogTitle id="delete-theme-dialog-title">
              {t('appearance.deleteTheme')}
            </DialogTitle>
          </DialogHeader>
          <div className="px-6">
            <p className="text-sm text-content-secondary">
              {t('appearance.confirmDeleteTheme')}
            </p>
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setConfirmDeleteId(null)}
            >
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => { void handleDelete() }}
              disabled={isDeleting}
              className="bg-error hover:bg-error/90 text-white"
            >
              {t('appearance.deleteTheme')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
