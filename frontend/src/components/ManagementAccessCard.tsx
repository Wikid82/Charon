import { ShieldCheck, XCircle } from 'lucide-react'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { accessListErrorMessage } from '../api/errorBody'
import { useAuth } from '../hooks/useAuth'
import { useManagementAccess, useSaveManagementAccess } from '../hooks/useSecurity'
import { Alert } from './ui/Alert'
import { Button } from './ui/Button'
import { Card } from './ui/Card'
import { Checkbox } from './ui/Checkbox'
import { Label } from './ui/Label'
import { Skeleton } from './ui/Skeleton'

import type { AccessList } from '../api/accessLists'

interface ManagementAccessCardProps {
  /** Every access list, so each one can be chosen or left out. */
  accessLists: AccessList[]
}

/**
 * Access Lists page card that chooses which access lists protect Charon's own interface. Lists on proxy hosts do not
 * apply here. Administrators choose the lists; other users see the chosen set. Notes are passive.
 */
export function ManagementAccessCard({ accessLists }: ManagementAccessCardProps) {
  const { t } = useTranslation()
  const { user } = useAuth()
  const isAdmin = user?.role === 'admin'
  const headingId = useId()
  const idPrefix = useId()
  const { data, isLoading, isError } = useManagementAccess()
  const saveMutation = useSaveManagementAccess()
  // Null until the administrator changes a box; the stored set is shown otherwise.
  const [draft, setDraft] = useState<string[] | null>(null)

  const stored = data?.lists.map((list) => list.uuid) ?? []
  const selected = draft ?? stored
  const saveError = saveMutation.error ? (accessListErrorMessage(saveMutation.error, t) ?? t('accessLists.managementAccess.saveFailed')) : null

  const toggle = (uuid: string, checked: boolean) => {
    setDraft((current) => {
      const base = (current ?? stored).filter((entry) => entry !== uuid)
      return checked ? [...base, uuid] : base
    })
  }

  const handleSave = () => {
    saveMutation.mutate(selected, { onSuccess: () => setDraft(null) })
  }

  const restrictionText = selected.length === 0
    ? t('accessLists.managementAccess.notRestricted')
    : t('accessLists.managementAccess.restricted')

  let body: React.ReactNode
  if (isLoading) {
    body = <Skeleton className="h-24 w-full" data-testid="management-access-skeleton" />
  } else if (isError || !data) {
    body = <p className="text-sm text-content-secondary">{t('accessLists.managementAccess.loadError')}</p>
  } else if (!isAdmin) {
    body = (
      <>
        <p className="text-sm font-medium text-content-primary">{restrictionText}</p>
        {data.lists.length > 0 && (
          <ul className="text-sm text-content-secondary space-y-1">
            {data.lists.map((list) => (
              <li key={list.uuid}>{list.name}</li>
            ))}
          </ul>
        )}
        <p className="text-sm text-content-muted">{t('permissions.adminOnlyError')}</p>
      </>
    )
  } else {
    body = (
      <form
        className="space-y-3"
        onSubmit={(event) => {
          event.preventDefault()
          handleSave()
        }}
      >
        <fieldset className="space-y-2" disabled={saveMutation.isPending}>
          <legend className="text-sm font-medium text-content-primary mb-1">
            {t('accessLists.managementAccess.fieldsetLabel')}
          </legend>
          {accessLists.length === 0 && (
            <p className="text-sm text-content-muted">{t('accessLists.managementAccess.noLists')}</p>
          )}
          {accessLists.map((list) => {
            const id = `${idPrefix}-${list.uuid}`
            const allows = list.type === 'whitelist' || list.type === 'geo_whitelist'
            return (
              <div key={list.uuid} className="flex items-start gap-2">
                <Checkbox
                  id={id}
                  checked={selected.includes(list.uuid)}
                  onCheckedChange={(checked) => toggle(list.uuid, checked === true)}
                />
                <Label htmlFor={id} className="cursor-pointer font-normal">
                  {list.name}
                  <span className="ml-2 text-xs text-content-muted">
                    {allows ? t('common.allow') : t('common.deny')}
                    {!list.enabled && ` · ${t('common.disabled')}`}
                  </span>
                </Label>
              </div>
            )
          })}
        </fieldset>
        <p className="text-sm text-content-secondary">{restrictionText}</p>
        {selected.length === 0 ? (
          <p className="text-xs text-content-muted">{t('accessLists.managementAccess.notRestrictedHint')}</p>
        ) : (
          !data.enforced && (
            <p className="text-xs text-content-muted">{t('accessLists.managementAccess.notApplied')}</p>
          )
        )}
        {saveError && (
          <Alert variant="error" icon={XCircle}>
            {saveError}
          </Alert>
        )}
        {saveMutation.isSuccess && !saveMutation.isPending && (
          <p role="status" className="text-sm text-content-secondary">
            {t('accessLists.managementAccess.saved')}
          </p>
        )}
        <Button type="submit" disabled={saveMutation.isPending}>
          {t('common.save')}
        </Button>
      </form>
    )
  }

  return (
    <section aria-labelledby={headingId}>
      <Card className="p-6 space-y-3">
        <div className="flex items-center gap-3">
          <ShieldCheck className="w-6 h-6 text-content-secondary" aria-hidden="true" />
          <h2 id={headingId} className="text-lg font-semibold text-content-primary">
            {t('accessLists.managementAccess.title')}
          </h2>
        </div>
        <p className="text-sm text-content-secondary">{t('accessLists.managementAccess.description')}</p>
        {body}
      </Card>
    </section>
  )
}
