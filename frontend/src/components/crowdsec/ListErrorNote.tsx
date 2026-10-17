import { Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CROWDSEC_LIST_ERROR_CODES, type CrowdsecListErrorCode } from '../../api/crowdsec'

interface ListErrorNoteProps {
  /** English soft-error text from the list response; shown when the code is unknown or absent. */
  error?: string
  error_code?: string
}

const isKnownCode = (code: string | undefined): code is CrowdsecListErrorCode =>
  CROWDSEC_LIST_ERROR_CODES.some((known) => known === code)

/**
 * Passive inline note for a CrowdSec list that came back partial or empty because
 * cscli could not answer. Renders nothing when the response carries no error.
 */
export function ListErrorNote({ error, error_code }: ListErrorNoteProps) {
  const { t } = useTranslation()
  if (!error && !error_code) return null

  const text = isKnownCode(error_code)
    ? t(`security.crowdsec.dashboard.listErrors.${error_code}`, error ?? error_code)
    : error ?? error_code

  return (
    <p
      role="note"
      data-testid="crowdsec-list-error-note"
      className="flex items-start gap-2 rounded-md border border-yellow-700/40 bg-yellow-900/10 px-3 py-2 text-xs text-yellow-200"
    >
      <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
      <span>{text}</span>
    </p>
  )
}
