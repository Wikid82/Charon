import { MessageSquarePlus, Bug, Sparkles, BookOpen } from 'lucide-react'
import { useState, useRef, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '../utils/cn'

const GITHUB_BUG_URL =
  'https://github.com/Wikid82/Charon/issues/new?template=bug_report.md'
const GITHUB_FEATURE_URL =
  'https://github.com/Wikid82/Charon/issues/new?template=feature_request.md'
const DOCS_URL = 'https://wikid82.github.io/Charon/'

export default function FeedbackWidget() {
  const { t } = useTranslation()
  const [isOpen, setIsOpen] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const firstLinkRef = useRef<HTMLAnchorElement>(null)

  useEffect(() => {
    if (!isOpen) return
    firstLinkRef.current?.focus()
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        setIsOpen(false)
        triggerRef.current?.focus()
      }
    }
    document.addEventListener('keydown', handleEscape)
    return () => document.removeEventListener('keydown', handleEscape)
  }, [isOpen])

  const close = () => {
    setIsOpen(false)
    triggerRef.current?.focus()
  }

  return (
    <div className="fixed bottom-6 right-6 z-50">
      {/* Panel and backdrop are only mounted when open — links never reach the tab
          order or screen-reader tree while the widget is closed. */}
      {isOpen && (
        <>
          {/* Backdrop — uses close() so focus always returns to the trigger */}
          <div
            className="fixed inset-0 z-10"
            aria-hidden="true"
            onClick={close}
          />

          <div className="absolute bottom-full right-0 mb-2 w-48 z-50">
            <nav
              id="feedback-panel"
              aria-label={t('feedback.panelLabel')}
              className={cn(
                'rounded-lg border shadow-lg overflow-hidden',
                'bg-white dark:bg-dark-card',
                'border-gray-200 dark:border-gray-800'
              )}
            >
              <a
                ref={firstLinkRef}
                href={GITHUB_BUG_URL}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={t('feedback.reportBugAriaLabel')}
                className={cn(
                  'flex items-center gap-3 px-4 py-3 text-sm',
                  'text-gray-700 dark:text-gray-300',
                  'hover:bg-gray-100 dark:hover:bg-gray-800',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
                  'transition-colors'
                )}
              >
                <Bug className="w-4 h-4 shrink-0" aria-hidden="true" />
                <div>
                  <div className="font-medium">{t('feedback.reportBug')}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400">
                    {t('feedback.reportBugDescription')}
                  </div>
                </div>
              </a>

              <a
                href={GITHUB_FEATURE_URL}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={t('feedback.requestFeatureAriaLabel')}
                className={cn(
                  'flex items-center gap-3 px-4 py-3 text-sm',
                  'border-t border-gray-200 dark:border-gray-800',
                  'text-gray-700 dark:text-gray-300',
                  'hover:bg-gray-100 dark:hover:bg-gray-800',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
                  'transition-colors'
                )}
              >
                <Sparkles className="w-4 h-4 shrink-0" aria-hidden="true" />
                <div>
                  <div className="font-medium">{t('feedback.requestFeature')}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400">
                    {t('feedback.requestFeatureDescription')}
                  </div>
                </div>
              </a>

              <a
                href={DOCS_URL}
                target="_blank"
                rel="noopener noreferrer"
                aria-label={t('feedback.viewDocsAriaLabel')}
                className={cn(
                  'flex items-center gap-3 px-4 py-3 text-sm',
                  'border-t border-gray-200 dark:border-gray-800',
                  'text-gray-700 dark:text-gray-300',
                  'hover:bg-gray-100 dark:hover:bg-gray-800',
                  'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2',
                  'transition-colors'
                )}
              >
                <BookOpen className="w-4 h-4 shrink-0" aria-hidden="true" />
                <div>
                  <div className="font-medium">{t('feedback.viewDocs')}</div>
                  <div className="text-xs text-gray-500 dark:text-gray-400">
                    {t('feedback.viewDocsDescription')}
                  </div>
                </div>
              </a>
            </nav>
          </div>
        </>
      )}

      {/* Trigger button */}
      <button
        ref={triggerRef}
        onClick={() => setIsOpen(prev => !prev)}
        aria-label={isOpen ? t('feedback.closeTriggerLabel') : t('feedback.triggerLabel')}
        aria-expanded={isOpen}
        aria-controls="feedback-panel"
        className={cn(
          'h-10 w-10 rounded-full flex items-center justify-center',
          'bg-brand-500 hover:bg-brand-600 text-white',
          'shadow-lg transition-colors',
          'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-2'
        )}
      >
        <MessageSquarePlus className="w-5 h-5" aria-hidden="true" />
      </button>
    </div>
  )
}
