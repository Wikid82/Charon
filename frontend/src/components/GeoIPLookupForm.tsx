import { useId, useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'

import { getApiErrorCode, geoipErrorKey, GEOIP_ERROR_CODES } from '../api/errorBody'
import { useGeoIPLookup, useMyIPAddress } from '../hooks/useGeoIP'
import { Button } from './ui/Button'
import { Input } from './ui/Input'
import { Label } from './ui/Label'

import type { GeoIPLookupResult } from '../api/security'
import type { TFunction } from 'i18next'

/** Display name of a region code, or the code itself when the runtime cannot name it. */
function countryName(code: string, language: string): string {
  try {
    return new Intl.DisplayNames([language], { type: 'region' }).of(code) ?? code
  } catch {
    return code
  }
}

/** Text of a finished lookup: the country, or that none was found for the address. */
function lookupResultText(result: GeoIPLookupResult, language: string, t: TFunction): string {
  if (!result.found) return t('security.geoip.lookup.notFound', { ip: result.ip_address })
  return t('security.geoip.lookup.found', {
    ip: result.ip_address,
    country: countryName(result.country_code, language),
  })
}

/** Text of a refused lookup. A 400 means the address did not parse; a 503 means no database is loaded. */
function lookupErrorText(error: unknown, t: TFunction): string {
  const status = (error as { response?: { status?: number } } | null)?.response?.status
  if (status === 400) return t('security.geoip.lookup.invalid')
  if (status === 503) return t('security.geoip.lookup.notLoaded')
  return t(geoipErrorKey(getApiErrorCode(error, GEOIP_ERROR_CODES)))
}

interface GeoIPLookupFormProps {
  /** False while no database is loaded: the form stays visible with a hint and cannot look up. */
  loaded: boolean
}

/** Checks which country an address belongs to, using the loaded database (admin only). */
export function GeoIPLookupForm({ loaded }: GeoIPLookupFormProps) {
  const { t, i18n } = useTranslation()
  const inputId = useId()
  const hintId = useId()
  const [address, setAddress] = useState('')
  const lookup = useGeoIPLookup()
  const myAddress = useMyIPAddress()

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    const ip = address.trim()
    if (ip === '' || !loaded) return
    lookup.mutate(ip)
  }

  const onUseMyAddress = () => {
    myAddress.mutate(undefined, {
      onSuccess: (result) => setAddress(result.ip),
    })
  }

  let resultText: string | null = null
  if (lookup.isError) resultText = lookupErrorText(lookup.error, t)
  else if (lookup.data) resultText = lookupResultText(lookup.data, i18n.language, t)

  return (
    <form onSubmit={onSubmit} className="space-y-2" aria-describedby={hintId}>
      <Label htmlFor={inputId}>{t('security.geoip.lookup.label')}</Label>
      <div className="flex flex-wrap gap-2">
        <Input
          id={inputId}
          className="flex-1 min-w-[12rem]"
          value={address}
          placeholder={t('security.geoip.lookup.placeholder')}
          onChange={(event) => setAddress(event.target.value)}
          disabled={!loaded}
          autoComplete="off"
          spellCheck={false}
        />
        <Button type="submit" variant="secondary" disabled={!loaded || address.trim() === '' || lookup.isPending}>
          {t('security.geoip.lookup.submit')}
        </Button>
        <Button type="button" variant="ghost" onClick={onUseMyAddress} disabled={myAddress.isPending}>
          {t('security.geoip.lookup.useMyAddress')}
        </Button>
      </div>
      <p id={hintId} className="text-xs text-content-muted">
        {loaded ? null : t('security.geoip.lookup.disabled')}
      </p>
      {myAddress.isError && <p className="text-sm text-error">{t('security.geoip.lookup.myAddressFailed')}</p>}
      <p aria-live="polite" data-testid="geoip-lookup-result" className="text-sm text-content-secondary">
        {resultText}
      </p>
    </form>
  )
}
