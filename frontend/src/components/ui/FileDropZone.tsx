import { Upload } from 'lucide-react'
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '../../utils/cn'

interface FileDropZoneProps {
  id: string
  label: string
  accept?: string
  file: File | null
  onFileChange: (file: File | null) => void
  disabled?: boolean
  required?: boolean
  formatBadge?: string | null
}

export function FileDropZone({
  id,
  label,
  accept,
  file,
  onFileChange,
  disabled = false,
  required = false,
  formatBadge,
}: FileDropZoneProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [isDragOver, setIsDragOver] = useState(false)

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setIsDragOver(false)
      if (disabled) return
      const droppedFile = e.dataTransfer.files[0]
      if (droppedFile) onFileChange(droppedFile)
    },
    [disabled, onFileChange],
  )

  const handleDragOver = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      if (!disabled) setIsDragOver(true)
    },
    [disabled],
  )

  const handleDragLeave = useCallback(() => {
    setIsDragOver(false)
  }, [])

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      const selected = e.target.files?.[0] || null
      onFileChange(selected)
    },
    [onFileChange],
  )

  const handleClick = useCallback(() => {
    if (!disabled) inputRef.current?.click()
  }, [disabled])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if ((e.key === 'Enter' || e.key === ' ') && !disabled) {
        e.preventDefault()
        inputRef.current?.click()
      }
    },
    [disabled],
  )

  return (
    <div>
      <label htmlFor={id} className="block text-sm font-medium text-content-secondary mb-1.5">
        {label}
        {required && <span className="text-error ml-0.5" aria-hidden="true">*</span>}
      </label>
      <div
        role="button"
        tabIndex={disabled ? -1 : 0}
        aria-label={file ? `${label}: ${file.name}` : label}
        aria-disabled={disabled}
        onClick={handleClick}
        onKeyDown={handleKeyDown}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        className={cn(
          'relative flex flex-col items-center justify-center rounded-lg border-2 border-dashed p-6 transition-colors cursor-pointer',
          'focus:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2 focus-visible:ring-offset-surface-elevated',
          isDragOver && !disabled
            ? 'border-brand-500 bg-brand-500/10'
            : 'border-gray-700 hover:border-gray-600 bg-surface-muted/30',
          disabled && 'opacity-50 cursor-not-allowed',
        )}
      >
        <input
          ref={inputRef}
          id={id}
          type="file"
          accept={accept}
          onChange={handleInputChange}
          disabled={disabled}
          className="sr-only"
          aria-required={required}
          required={required}
          tabIndex={-1}
        />

        {file ? (
          <div className="flex items-center gap-2 text-sm">
            <Upload className="h-4 w-4 text-brand-400" aria-hidden="true" />
            <span className="text-content-primary font-medium truncate max-w-[200px]">
              {file.name}
            </span>
            {formatBadge && (
              <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-brand-500/20 text-brand-400 border border-brand-500/30">
                {formatBadge}
              </span>
            )}
          </div>
        ) : (
          <div className="flex flex-col items-center gap-1 text-sm text-content-muted">
            <Upload className="h-5 w-5 mb-1" aria-hidden="true" />
            <span>{t('certificates.dropFileHere')}</span>
          </div>
        )}
      </div>
    </div>
  )
}
