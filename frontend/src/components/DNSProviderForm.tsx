import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { ChevronDown, ChevronUp, ExternalLink, CheckCircle, XCircle, Settings } from 'lucide-react'
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Button,
  Input,
  Label,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Checkbox,
  Alert,
} from './ui'
import { useDNSProviderTypes, useDNSProviderMutations, type DNSProvider } from '../hooks/useDNSProviders'
import type { DNSProviderRequest, DNSProviderTypeInfo } from '../api/dnsProviders'
import { defaultProviderSchemas } from '../data/dnsProviderSchemas'
import { useEnableMultiCredentials, useCredentials } from '../hooks/useCredentials'
import { useProviderFields } from '../hooks/usePlugins'
import CredentialManager from './CredentialManager'

interface DNSProviderFormProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  provider?: DNSProvider | null
  onSuccess: () => void
}

export default function DNSProviderForm({
  open,
  onOpenChange,
  provider = null,
  onSuccess,
}: DNSProviderFormProps) {
  const { t } = useTranslation()
  const { data: providerTypes, isLoading: typesLoading } = useDNSProviderTypes()
  const { createMutation, updateMutation, testCredentialsMutation } = useDNSProviderMutations()
  const enableMultiCredsMutation = useEnableMultiCredentials()
  const { data: existingCredentials } = useCredentials(provider?.id || 0)

  const [name, setName] = useState('')
  const [providerType, setProviderType] = useState<string>('')
  const { data: dynamicFields } = useProviderFields(providerType)
  const [credentials, setCredentials] = useState<Record<string, string>>({})
  const [propagationTimeout, setPropagationTimeout] = useState(120)
  const [pollingInterval, setPollingInterval] = useState(5)
  const [isDefault, setIsDefault] = useState(false)
  const [useMultiCredentials, setUseMultiCredentials] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [showCredentialManager, setShowCredentialManager] = useState(false)
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null)

  // Populate form when editing
  useEffect(() => {
    if (provider) {
      setName(provider.name)
      setProviderType(provider.provider_type)
      setPropagationTimeout(provider.propagation_timeout)
      setPollingInterval(provider.polling_interval)
      setIsDefault(provider.is_default)
      setUseMultiCredentials((provider as any).use_multi_credentials || false)
      setCredentials({}) // Don't pre-fill credentials (they're encrypted)
    } else {
      resetForm()
    }
  }, [provider, open])

  const resetForm = () => {
    setName('')
    setProviderType('')
    setCredentials({})
    setPropagationTimeout(120)
    setPollingInterval(5)
    setIsDefault(false)
    setUseMultiCredentials(false)
    setShowAdvanced(false)
    setShowCredentialManager(false)
    setTestResult(null)
  }

  const getSelectedProviderInfo = (): DNSProviderTypeInfo | undefined => {
    if (!providerType) return undefined

    // Prefer dynamic fields from API if available
    if (dynamicFields) {
      return {
        type: dynamicFields.type as any,
        name: dynamicFields.name,
        fields: [
          ...dynamicFields.required_fields.map(f => ({ ...f, required: true })),
          ...dynamicFields.optional_fields.map(f => ({ ...f, required: false })),
        ],
        documentation_url: '',
      }
    }

    // Fallback to static types or schemas
    return (
      providerTypes?.find((pt) => pt.type === providerType) ||
      (defaultProviderSchemas[providerType as keyof typeof defaultProviderSchemas] as DNSProviderTypeInfo)
    )
  }

  const handleCredentialChange = (fieldName: string, value: string) => {
    setCredentials((prev) => ({ ...prev, [fieldName]: value }))
  }

  const handleTestConnection = async () => {
    const selectedProvider = getSelectedProviderInfo()
    if (!selectedProvider) return

    const data: DNSProviderRequest = {
      name: name || 'Test',
      provider_type: providerType as any,
      credentials,
      propagation_timeout: propagationTimeout,
      polling_interval: pollingInterval,
    }

    try {
      const result = await testCredentialsMutation.mutateAsync(data)
      setTestResult({
        success: result.success,
        message: result.message || result.error || t('dnsProviders.testSuccess'),
      })
    } catch (error: any) {
      setTestResult({
        success: false,
        message: error.response?.data?.error || error.message || t('dnsProviders.testFailed'),
      })
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setTestResult(null)

    const data: DNSProviderRequest = {
      name,
      provider_type: providerType as any,
      credentials,
      propagation_timeout: propagationTimeout,
      polling_interval: pollingInterval,
      is_default: isDefault,
    }

    try {
      if (provider) {
        await updateMutation.mutateAsync({ id: provider.id, data })
      } else {
        await createMutation.mutateAsync(data)
      }
      onSuccess()
      onOpenChange(false)
      resetForm()
    } catch (error) {
      console.error('Failed to save DNS provider:', error)
    }
  }

  const selectedProviderInfo = getSelectedProviderInfo()
  const isSubmitting = createMutation.isPending || updateMutation.isPending
  const isTesting = testCredentialsMutation.isPending

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {provider ? t('dnsProviders.editProvider') : t('dnsProviders.addProvider')}
          </DialogTitle>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 py-4">
          {/* Provider Type */}
          <div>
            <Label htmlFor="provider-type">{t('dnsProviders.providerType')}</Label>
            <Select
              value={providerType}
              onValueChange={setProviderType}
              disabled={!!provider} // Can't change type when editing
            >
              <SelectTrigger id="provider-type">
                <SelectValue placeholder={t('dnsProviders.selectProviderType')} />
              </SelectTrigger>
              <SelectContent>
                {typesLoading ? (
                  <SelectItem value="loading" disabled>
                    {t('common.loading')}
                  </SelectItem>
                ) : (
                  (providerTypes || Object.values(defaultProviderSchemas)).map((type) => (
                    <SelectItem key={type.type} value={type.type!}>
                      {type.name}
                    </SelectItem>
                  ))
                )}
              </SelectContent>
            </Select>
          </div>

          {/* Provider Name */}
          <Input
            label={t('dnsProviders.providerName')}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder={t('dnsProviders.providerNamePlaceholder')}
            required
          />

          {/* Dynamic Credential Fields */}
          {selectedProviderInfo && (
            <>
              <div className="space-y-3 border-t pt-4">
                <div className="flex items-center justify-between">
                  <Label className="text-base">{t('dnsProviders.credentials')}</Label>
                  {selectedProviderInfo.documentation_url && (
                    <a
                      href={selectedProviderInfo.documentation_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm text-brand-500 hover:text-brand-600 flex items-center gap-1"
                    >
                      {t('dnsProviders.viewDocs')}
                      <ExternalLink className="w-3 h-3" />
                    </a>
                  )}
                </div>

                {selectedProviderInfo.fields?.map((field) => (
                  <Input
                    key={field.name}
                    label={field.label}
                    type={field.type}
                    value={credentials[field.name] || ''}
                    onChange={(e) => handleCredentialChange(field.name, e.target.value)}
                    placeholder={field.default}
                    helperText={field.hint}
                    required={field.required && !provider} // Don't require when editing (preserve existing)
                  />
                ))}
              </div>

              {/* Test Connection */}
              <div className="space-y-2">
                <Button
                  type="button"
                  variant="secondary"
                  onClick={handleTestConnection}
                  isLoading={isTesting}
                  disabled={!providerType || !name}
                  className="w-full"
                >
                  {t('dnsProviders.testConnection')}
                </Button>

                {testResult && (
                  <Alert variant={testResult.success ? 'success' : 'error'}>
                    <div className="flex items-start gap-2">
                      {testResult.success ? (
                        <CheckCircle className="w-4 h-4 mt-0.5" />
                      ) : (
                        <XCircle className="w-4 h-4 mt-0.5" />
                      )}
                      <div>
                        <p className="font-medium">
                          {testResult.success
                            ? t('dnsProviders.testSuccess')
                            : t('dnsProviders.testFailed')}
                        </p>
                        <p className="text-sm mt-1">{testResult.message}</p>
                      </div>
                    </div>
                  </Alert>
                )}
              </div>

              {/* Multi-Credential Mode (only when editing) */}
              {provider && (
                <div className="border-t pt-4 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <Checkbox
                        id="use-multi-credentials"
                        checked={useMultiCredentials}
                        onCheckedChange={async (checked) => {
                          if (checked && !useMultiCredentials) {
                            // Enabling multi-credential mode
                            try {
                              await enableMultiCredsMutation.mutateAsync(provider.id)
                              setUseMultiCredentials(true)
                            } catch (error: any) {
                              console.error('Failed to enable multi-credentials:', error)
                            }
                          } else if (!checked && useMultiCredentials && existingCredentials?.length) {
                            // Warn before disabling if credentials exist
                            if (
                              !confirm(
                                t(
                                  'credentials.disableWarning',
                                  'Disabling multi-credential mode will remove all configured credentials. Continue?'
                                )
                              )
                            ) {
                              return
                            }
                            setUseMultiCredentials(false)
                          } else {
                            setUseMultiCredentials(checked === true)
                          }
                        }}
                        disabled={enableMultiCredsMutation.isPending}
                      />
                      <Label htmlFor="use-multi-credentials" className="cursor-pointer">
                        {t('credentials.useMultiCredentials', 'Use Multiple Credentials (Advanced)')}
                      </Label>
                    </div>
                    {useMultiCredentials && (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        onClick={() => setShowCredentialManager(true)}
                      >
                        <Settings className="w-4 h-4 mr-2" />
                        {t('credentials.manageCredentials', 'Manage Credentials')}
                      </Button>
                    )}
                  </div>
                  {useMultiCredentials && (
                    <Alert variant="info">
                      <p className="text-sm">
                        {t(
                          'credentials.multiCredentialInfo',
                          'Multi-credential mode allows you to configure different credentials for specific zones or domains.'
                        )}
                      </p>
                    </Alert>
                  )}
                </div>
              )}
            </>
          )}

          {/* Advanced Settings */}
          <div className="border-t pt-4">
            <button
              type="button"
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-2 text-sm font-medium text-content-secondary hover:text-content-primary transition-colors"
            >
              {showAdvanced ? (
                <ChevronUp className="w-4 h-4" />
              ) : (
                <ChevronDown className="w-4 h-4" />
              )}
              {t('dnsProviders.advancedSettings')}
            </button>

            {showAdvanced && (
              <div className="mt-4 space-y-3">
                <Input
                  label={t('dnsProviders.propagationTimeout')}
                  type="number"
                  value={propagationTimeout}
                  onChange={(e) => setPropagationTimeout(parseInt(e.target.value, 10))}
                  helperText={t('dnsProviders.propagationTimeoutHint')}
                  min={30}
                  max={600}
                />
                <Input
                  label={t('dnsProviders.pollingInterval')}
                  type="number"
                  value={pollingInterval}
                  onChange={(e) => setPollingInterval(parseInt(e.target.value, 10))}
                  helperText={t('dnsProviders.pollingIntervalHint')}
                  min={1}
                  max={60}
                />
                <div className="flex items-center gap-2">
                  <Checkbox
                    id="is-default"
                    checked={isDefault}
                    onCheckedChange={(checked) => setIsDefault(checked as boolean)}
                  />
                  <Label htmlFor="is-default" className="cursor-pointer">
                    {t('dnsProviders.setAsDefault')}
                  </Label>
                </div>
              </div>
            )}
          </div>

          {/* Form Actions */}
          <DialogFooter className="pt-4">
            <Button
              type="button"
              variant="secondary"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting}
            >
              {t('common.cancel')}
            </Button>
            <Button type="submit" isLoading={isSubmitting} disabled={!providerType || !name}>
              {provider ? t('common.update') : t('common.create')}
            </Button>
          </DialogFooter>
        </form>

        {/* Credential Manager Modal */}
        {provider && showCredentialManager && (
          <CredentialManager
            open={showCredentialManager}
            onOpenChange={setShowCredentialManager}
            provider={provider}
            providerTypeInfo={selectedProviderInfo}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}
