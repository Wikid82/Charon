import { useTranslation } from 'react-i18next'

import { useAuth } from '../hooks/useAuth'
import { useAccessListSelfCheck, useGeoIPStatus } from '../hooks/useGeoIP'

import type { AccessListSelfCheckDraft } from '../api/accessLists'

interface AccessListGeoNoteProps {
  /** The country list as it is currently edited; nothing here is saved. */
  draft: AccessListSelfCheckDraft
}

/**
 * Passive, muted line under a country list's form fields. It says the list is not enforced while no GeoIP database
 * is usable, or that the viewer's current address would be refused. It never blocks saving.
 *
 * The self-check is admin-only, so for other users only the database status is considered.
 */
export function AccessListGeoNote({ draft }: AccessListGeoNoteProps) {
  const { t } = useTranslation()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const { data: status } = useGeoIPStatus()
  const { data: selfCheck } = useAccessListSelfCheck(draft, isAdmin && draft.country_codes !== '')

  let message: string | null = null
  if (status?.available === false) {
    message = t('security.geoip.formNote.unavailable')
  } else if (selfCheck && !selfCheck.allowed && !selfCheck.admin_exempt) {
    message = t('security.geoip.formNote.wouldRefuseYou')
  }

  if (message === null) return null

  return (
    <p role="status" className="text-xs text-content-muted">
      {message}
    </p>
  )
}
