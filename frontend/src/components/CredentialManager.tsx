import { Plus, Edit, Trash2, CheckCircle, XCircle, TestTube } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Button,
  Input,
  Label,
  Checkbox,
  EmptyState,
} from './ui'
import {
  useCredentials,
  useCreateCredential,
  useUpdateCredential,
  useDeleteCredential,
  useTestCredential,
  type DNSProviderCredential,
  type CredentialRequest,
} from '../hooks/useCredentials'
import { toast } from '../utils/toast'

import type { DNSProvider, DNSProviderTypeInfo } from '../api/dnsProviders'

interface CredentialManagerProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  provider: DNSProvider
  providerTypeInfo?: DNSProviderTypeInfo
}

export default function CredentialManager({
  open,
  onOpenChange,
  provider,
  providerTypeInfo,
}: CredentialManagerProps) {
  const { t } = useTranslation()
  const { data: credentials = [], isLoading, refetch } = useCredentials(provider.id)
  const deleteMutation = useDeleteCredential()
  const testMutation = useTestCredential()

  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingCredential, setEditingCredential] = useState<DNSProviderCredential | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<number | null>(null)
  const [testingId, setTestingId] = useState<number | null>(null)

  const handleAddCredential = () => {
    setEditingCredential(null)
    setIsFormOpen(true)
  }

  const handleEditCredential = (credential: DNSProviderCredential) => {
    setEditingCredential(credential)
    setIsFormOpen(true)
  }

  const handleDeleteClick = (id: number) => {
    setDeleteConfirm(id)
  }

  const handleDeleteConfirm = async (id: number) => {
    try {
      await deleteMutation.mutateAsync({ providerId: provider.id, credentialId: id })
      toast.success(t('credentials.deleteSuccess', 'Credential deleted successfully'))
      setDeleteConfirm(null)
      refetch()
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      toast.error(
        t('credentials.deleteFailed', 'Failed to delete credential') +
          ': ' +
          (err.response?.data?.error || err.message)
      )
    }
  }

  const handleTestCredential = async (id: number) => {
    setTestingId(id)
    try {
      const result = await testMutation.mutateAsync({
        providerId: provider.id,
        credentialId: id,
      })
      if (result.success) {
        toast.success(result.message || t('credentials.testSuccess', 'Credential test passed'))
      } else {
        toast.error(result.error || t('credentials.testFailed', 'Credential test failed'))
      }
      refetch()
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      toast.error(
        t('credentials.testFailed', 'Failed to test credential') +
          ': ' +
          (err.response?.data?.error || err.message)
      )
    } finally {
      setTestingId(null)
    }
  }

  const handleFormSuccess = () => {
    toast.success(
      editingCredential
        ? t('credentials.updateSuccess', 'Credential updated successfully')
        : t('credentials.createSuccess', 'Credential created successfully')
    )
    setIsFormOpen(false)
    refetch()
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-5xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {t('credentials.manageTitle', 'Manage Credentials')}: {provider.name}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Add Button */}
          <div className="flex justify-between items-center">
            <Button onClick={handleAddCredential} size="sm">
              <Plus className="w-4 h-4 mr-2" />
              {t('credentials.addCredential', 'Add Credential')}
            </Button>
          </div>

          {/* Loading State */}
          {isLoading && (
            <div className="text-center py-8 text-muted-foreground">
              {t('common.loading', 'Loading...')}
            </div>
          )}

          {/* Empty State */}
          {!isLoading && credentials.length === 0 && (
            <EmptyState
              icon={<CheckCircle className="w-10 h-10" />}
              title={t('credentials.noCredentials', 'No credentials configured')}
              description={t(
                'credentials.noCredentialsDescription',
                'Add credentials to enable zone-specific DNS challenge configuration'
              )}
              action={{
                label: t('credentials.addFirst', 'Add First Credential'),
                onClick: handleAddCredential,
              }}
            />
          )}

          {/* Credentials Table */}
          {!isLoading && credentials.length > 0 && (
            <div className="border rounded-lg overflow-hidden">
              <table className="w-full">
                <thead className="bg-muted">
                  <tr>
                    <th className="px-4 py-3 text-left text-sm font-medium">
                      {t('credentials.label', 'Label')}
                    </th>
                    <th className="px-4 py-3 text-left text-sm font-medium">
                      {t('credentials.zones', 'Zones')}
                    </th>
                    <th className="px-4 py-3 text-left text-sm font-medium">
                      {t('credentials.status', 'Status')}
                    </th>
                    <th className="px-4 py-3 text-right text-sm font-medium">
                      {t('common.actions', 'Actions')}
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y">
                  {credentials.map((credential) => (
                    <tr key={credential.id} className="hover:bg-muted/50">
                      <td className="px-4 py-3">
                        <div className="font-medium">{credential.label}</div>
                        {!credential.enabled && (
                          <span className="text-xs text-muted-foreground">
                            {t('common.disabled', 'Disabled')}
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-sm">
                        {credential.zone_filter || (
                          <span className="text-muted-foreground italic">
                            {t('credentials.allZones', 'All zones (catch-all)')}
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex items-center gap-2">
                          {credential.failure_count > 0 ? (
                            <XCircle className="w-4 h-4 text-destructive" />
                          ) : (
                            <CheckCircle className="w-4 h-4 text-success" />
                          )}
                          <span className="text-sm">
                            {credential.success_count}/{credential.failure_count}
                          </span>
                        </div>
                        {credential.last_used_at && (
                          <div className="text-xs text-muted-foreground">
                            {t('credentials.lastUsed', 'Last used')}:{' '}
                            {new Date(credential.last_used_at).toLocaleString()}
                          </div>
                        )}
                        {credential.last_error && (
                          <div className="text-xs text-destructive mt-1">
                            {credential.last_error}
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <div className="flex justify-end gap-2">
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => handleTestCredential(credential.id)}
                            disabled={testingId === credential.id}
                          >
                            <TestTube className="w-4 h-4" />
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => handleEditCredential(credential)}
                          >
                            <Edit className="w-4 h-4" />
                          </Button>
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => handleDeleteClick(credential.id)}
                            className="text-destructive hover:text-destructive"
                          >
                            <Trash2 className="w-4 h-4" />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.close', 'Close')}
          </Button>
        </DialogFooter>
      </DialogContent>

      {/* Credential Form Dialog */}
      {isFormOpen && (
        <CredentialForm
          open={isFormOpen}
          onOpenChange={setIsFormOpen}
          providerId={provider.id}
          providerTypeInfo={providerTypeInfo}
          credential={editingCredential}
          onSuccess={handleFormSuccess}
        />
      )}

      {/* Delete Confirmation Dialog */}
      {deleteConfirm !== null && (
        <Dialog open={true} onOpenChange={() => setDeleteConfirm(null)}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t('credentials.deleteConfirm', 'Delete Credential?')}</DialogTitle>
            </DialogHeader>
            <p className="text-sm text-muted-foreground">
              {t(
                'credentials.deleteWarning',
                'Are you sure you want to delete this credential? This action cannot be undone.'
              )}
            </p>
            <DialogFooter>
              <Button variant="outline" onClick={() => setDeleteConfirm(null)}>
                {t('common.cancel', 'Cancel')}
              </Button>
              <Button
                variant="danger"
                onClick={() => handleDeleteConfirm(deleteConfirm)}
                disabled={deleteMutation.isPending}
              >
                {t('common.delete', 'Delete')}
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      )}
    </Dialog>
  )
}

interface CredentialFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  providerId: number
  providerTypeInfo?: DNSProviderTypeInfo
  credential: DNSProviderCredential | null
  onSuccess: () => void
}

function CredentialForm({
  open,
  onOpenChange,
  providerId,
  providerTypeInfo,
  credential,
  onSuccess,
}: CredentialFormProps) {
  const { t } = useTranslation()
  const createMutation = useCreateCredential()
  const updateMutation = useUpdateCredential()
  const testMutation = useTestCredential()

  const [label, setLabel] = useState('')
  const [zoneFilter, setZoneFilter] = useState('')
  const [credentials, setCredentials] = useState<Record<string, string>>({})
  const [propagationTimeout, setPropagationTimeout] = useState(120)
  const [pollingInterval, setPollingInterval] = useState(5)
  const [enabled, setEnabled] = useState(true)
  const [errors, setErrors] = useState<Record<string, string>>({})

  useEffect(() => {
    if (credential) {
      setLabel(credential.label)
      setZoneFilter(credential.zone_filter)
      setPropagationTimeout(credential.propagation_timeout)
      setPollingInterval(credential.polling_interval)
      setEnabled(credential.enabled)
      setCredentials({}) // Don't pre-fill credentials (they're encrypted)
    } else {
      resetForm()
    }
  }, [credential, open])

  const resetForm = () => {
    setLabel('')
    setZoneFilter('')
    setCredentials({})
    setPropagationTimeout(120)
    setPollingInterval(5)
    setEnabled(true)
    setErrors({})
  }

  const validateZoneFilter = (value: string): boolean => {
    if (!value) return true // Empty is valid (catch-all)

    const zones = value.split(',').map((z) => z.trim())
    for (const zone of zones) {
      // Basic domain validation
      // Runs client-side only; ReDoS risk is confined to the user's own browser session
      // eslint-disable-next-line security/detect-unsafe-regex
      if (zone && !/^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/.test(zone)) {
        setErrors((prev) => ({
          ...prev,
          zone_filter: t('credentials.invalidZone', 'Invalid domain format: ') + zone,
        }))
        return false
      }
    }
    setErrors((prev) => {
      const { zone_filter: _, ...rest } = prev
      return rest
    })
    return true
  }

  const handleCredentialChange = (fieldName: string, value: string) => {
    setCredentials((prev) => ({ ...prev, [fieldName]: value }))
  }

  const handleSubmit = async () => {
    // Validate
    if (!label.trim()) {
      setErrors({ label: t('credentials.labelRequired', 'Label is required') })
      return
    }

    if (!validateZoneFilter(zoneFilter)) {
      return
    }

    // Check required credential fields
    const missingFields: string[] = []
    for (const field of (providerTypeInfo?.fields ?? [])
      .filter((f) => f.required)) {
        if (!credentials[field.name]) {
          missingFields.push(field.label)
        }
      }

    if (missingFields.length > 0 && !credential) {
      // Only enforce for new credentials
      toast.error(
        t('credentials.missingFields', 'Missing required fields: ') + missingFields.join(', ')
      )
      return
    }

    const data: CredentialRequest = {
      label: label.trim(),
      zone_filter: zoneFilter.trim(),
      credentials,
      propagation_timeout: propagationTimeout,
      polling_interval: pollingInterval,
      enabled,
    }

    try {
      if (credential) {
        await updateMutation.mutateAsync({
          providerId,
          credentialId: credential.id,
          data,
        })
      } else {
        await createMutation.mutateAsync({ providerId, data })
      }
      onSuccess()
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      toast.error(
        t('credentials.saveFailed', 'Failed to save credential') +
          ': ' +
          (err.response?.data?.error || err.message)
      )
    }
  }

  const handleTest = async () => {
    if (!credential) {
      toast.info(t('credentials.saveBeforeTest', 'Please save the credential before testing'))
      return
    }

    try {
      const result = await testMutation.mutateAsync({
        providerId,
        credentialId: credential.id,
      })
      if (result.success) {
        toast.success(result.message || t('credentials.testSuccess', 'Test passed'))
      } else {
        toast.error(result.error || t('credentials.testFailed', 'Test failed'))
      }
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      toast.error(
        t('credentials.testFailed', 'Test failed') +
          ': ' +
          (err.response?.data?.error || err.message)
      )
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {credential
              ? t('credentials.editCredential', 'Edit Credential')
              : t('credentials.addCredential', 'Add Credential')}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Label */}
          <div>
            <Label htmlFor="label">
              {t('credentials.label', 'Label')} <span className="text-destructive">*</span>
            </Label>
            <Input
              id="label"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              placeholder={t('credentials.labelPlaceholder', 'e.g., Production, Customer A')}
              error={errors.label}
            />
          </div>

          {/* Zone Filter */}
          <div>
            <Label htmlFor="zone_filter">{t('credentials.zoneFilter', 'Zone Filter')}</Label>
            <Input
              id="zone_filter"
              value={zoneFilter}
              onChange={(e) => {
                setZoneFilter(e.target.value)
                validateZoneFilter(e.target.value)
              }}
              placeholder="example.com, *.staging.example.com"
              error={errors.zone_filter}
            />
            <p className="text-xs text-muted-foreground mt-1">
              {t(
                'credentials.zoneFilterHint',
                'Comma-separated domains. Leave empty for catch-all. Supports wildcards (*.example.com)'
              )}
            </p>
          </div>

          {/* Credentials Fields */}
          {providerTypeInfo?.fields.map((field) => (
            <div key={field.name}>
              <Label htmlFor={field.name}>
                {field.label} {field.required && <span className="text-destructive">*</span>}
              </Label>
              <Input
                id={field.name}
                type={field.type === 'password' ? 'password' : 'text'}
                value={credentials[field.name] || ''}
                onChange={(e) => handleCredentialChange(field.name, e.target.value)}
                placeholder={
                  credential
                    ? t('credentials.leaveBlankToKeep', '(leave blank to keep existing)')
                    : field.default || ''
                }
              />
              {field.hint && (
                <p className="text-xs text-muted-foreground mt-1">{field.hint}</p>
              )}
            </div>
          ))}

          {/* Enabled Checkbox */}
          <div className="flex items-center gap-2">
            <Checkbox
              id="enabled"
              checked={enabled}
              onCheckedChange={(checked) => setEnabled(checked === true)}
            />
            <Label htmlFor="enabled" className="cursor-pointer">
              {t('credentials.enabled', 'Enabled')}
            </Label>
          </div>

          {/* Advanced Options */}
          <details className="border rounded-lg p-4">
            <summary className="cursor-pointer font-medium">
              {t('common.advancedOptions', 'Advanced Options')}
            </summary>
            <div className="space-y-4 mt-4">
              <div>
                <Label htmlFor="propagation_timeout">
                  {t('dnsProviders.propagationTimeout', 'Propagation Timeout (seconds)')}
                </Label>
                <Input
                  id="propagation_timeout"
                  type="number"
                  min="10"
                  max="600"
                  value={propagationTimeout}
                  onChange={(e) => setPropagationTimeout(parseInt(e.target.value) || 120)}
                />
              </div>
              <div>
                <Label htmlFor="polling_interval">
                  {t('dnsProviders.pollingInterval', 'Polling Interval (seconds)')}
                </Label>
                <Input
                  id="polling_interval"
                  type="number"
                  min="1"
                  max="60"
                  value={pollingInterval}
                  onChange={(e) => setPollingInterval(parseInt(e.target.value) || 5)}
                />
              </div>
            </div>
          </details>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t('common.cancel', 'Cancel')}
          </Button>
          {credential && (
            <Button
              variant="secondary"
              onClick={handleTest}
              disabled={testMutation.isPending}
            >
              {t('common.test', 'Test')}
            </Button>
          )}
          <Button
            onClick={handleSubmit}
            disabled={createMutation.isPending || updateMutation.isPending}
          >
            {t('common.save', 'Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
