import { Download, ChevronDown, FileJson, FileSpreadsheet, Loader2 } from 'lucide-react'
import { useState, useRef, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'

import { exportDecisions, type TimeRange } from '../../api/crowdsecDashboard'

interface DecisionsExportButtonProps {
  range: TimeRange
}

type ExportFormat = 'csv' | 'json'

export function DecisionsExportButton({ range }: DecisionsExportButtonProps) {
  const { t } = useTranslation()
  const [isOpen, setIsOpen] = useState(false)
  const [isExporting, setIsExporting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)
  const menuItemsRef = useRef<(HTMLButtonElement | null)[]>([])
  const [focusedIndex, setFocusedIndex] = useState(-1)

  const closeMenu = useCallback(() => {
    setIsOpen(false)
    setFocusedIndex(-1)
    buttonRef.current?.focus()
  }, [])

  useEffect(() => {
    if (!isOpen) return

    const handleClickOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        closeMenu()
      }
    }

    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [isOpen, closeMenu])

  useEffect(() => {
    if (isOpen && focusedIndex >= 0 && menuItemsRef.current[focusedIndex]) {
      menuItemsRef.current[focusedIndex]?.focus()
    }
  }, [focusedIndex, isOpen])

  const handleToggle = () => {
    if (isOpen) {
      closeMenu()
    } else {
      setIsOpen(true)
      setFocusedIndex(0)
      setError(null)
    }
  }

  const handleMenuKeyDown = (e: React.KeyboardEvent) => {
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault()
        setFocusedIndex((i) => Math.min(i + 1, 1))
        break
      case 'ArrowUp':
        e.preventDefault()
        setFocusedIndex((i) => Math.max(i - 1, 0))
        break
      case 'Escape':
        e.preventDefault()
        closeMenu()
        break
      case 'Tab':
        closeMenu()
        break
    }
  }

  const handleButtonKeyDown = (e: React.KeyboardEvent) => {
    if (!isOpen && (e.key === 'ArrowDown' || e.key === 'Enter' || e.key === ' ')) {
      e.preventDefault()
      setIsOpen(true)
      setFocusedIndex(0)
      setError(null)
    }
  }

  const handleExport = async (format: ExportFormat) => {
    closeMenu()
    setIsExporting(true)
    setError(null)
    try {
      const blob = await exportDecisions(format, range)
      if (!blob || blob.size === 0) {
        throw new Error('Empty response')
      }
      const timestamp = new Date().toISOString().slice(0, 10)
      const filename = `crowdsec-decisions-${timestamp}.${format}`
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = filename
      document.body.appendChild(a)
      a.click()
      a.remove()
      window.URL.revokeObjectURL(url)
    } catch {
      setError(t('security.crowdsec.dashboard.exportError', 'Export failed. Please try again.'))
    } finally {
      setIsExporting(false)
    }
  }

  const menuItems: { format: ExportFormat; label: string; icon: typeof FileJson }[] = [
    {
      format: 'csv',
      label: t('security.crowdsec.dashboard.exportCSV', 'Export as CSV'),
      icon: FileSpreadsheet,
    },
    {
      format: 'json',
      label: t('security.crowdsec.dashboard.exportJSON', 'Export as JSON'),
      icon: FileJson,
    },
  ]

  return (
    <div className="relative" ref={menuRef} data-testid="decisions-export">
      <button
        ref={buttonRef}
        type="button"
        onClick={handleToggle}
        onKeyDown={handleButtonKeyDown}
        disabled={isExporting}
        className="inline-flex items-center gap-2 rounded-md bg-gray-800 px-3 py-2 text-sm text-gray-300 hover:bg-gray-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 focus-visible:ring-offset-2 focus-visible:ring-offset-gray-950 disabled:opacity-50 disabled:cursor-not-allowed"
        aria-haspopup="true"
        aria-expanded={isOpen}
        aria-controls="export-menu"
        aria-label={t('security.crowdsec.dashboard.exportDecisions', 'Export decisions')}
      >
        {isExporting ? (
          <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" />
        ) : (
          <Download className="h-4 w-4" aria-hidden="true" />
        )}
        {t('security.crowdsec.dashboard.export', 'Export')}
        <ChevronDown className="h-3 w-3" aria-hidden="true" />
      </button>

      {isOpen && (
        <div
          role="menu"
          id="export-menu"
          tabIndex={-1}
          aria-label={t('security.crowdsec.dashboard.exportFormat', 'Export format')}
          className="absolute right-0 z-10 mt-1 w-48 rounded-md border border-gray-700 bg-gray-900 shadow-lg"
          onKeyDown={handleMenuKeyDown}
        >
          {menuItems.map((item, index) => {
            const Icon = item.icon
            return (
              <button
                key={item.format}
                ref={(el) => { menuItemsRef.current[index] = el }}
                type="button"
                role="menuitem"
                tabIndex={focusedIndex === index ? 0 : -1}
                className="flex w-full items-center gap-2 px-3 py-2 text-sm text-gray-300 hover:bg-gray-800 focus:outline-none focus-visible:bg-gray-800 first:rounded-t-md last:rounded-b-md"
                onClick={() => handleExport(item.format)}
              >
                <Icon className="h-4 w-4" aria-hidden="true" />
                {item.label}
              </button>
            )
          })}
        </div>
      )}

      {error && (
        <p className="absolute right-0 mt-1 text-xs text-red-400 whitespace-nowrap" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}
