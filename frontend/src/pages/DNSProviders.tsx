import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, Cloud } from 'lucide-react'
import { Button, Alert, EmptyState, Skeleton } from '../components/ui'
import DNSProviderCard from '../components/DNSProviderCard'
import DNSProviderForm from '../components/DNSProviderForm'
import { ManualDNSChallenge } from '../components/dns-providers'
import { useDNSProviders, useDNSProviderMutations, type DNSProvider } from '../hooks/useDNSProviders'
import { getChallenge, type ManualChallenge } from '../api/manualChallenge'
import { toast } from '../utils/toast'

export default function DNSProviders() {
  const { t } = useTranslation()
  const { data: providers = [], isLoading, refetch } = useDNSProviders()
  const { deleteMutation, testMutation } = useDNSProviderMutations()

  const [isFormOpen, setIsFormOpen] = useState(false)
  const [editingProvider, setEditingProvider] = useState<DNSProvider | null>(null)
  const [testingProviderId, setTestingProviderId] = useState<number | null>(null)
  const [manualChallenge, setManualChallenge] = useState<ManualChallenge | null>(null)
  const [activeManualProviderId, setActiveManualProviderId] = useState<number | null>(null)

  const manualProviderId = providers.find((provider) => provider.provider_type === 'manual')?.id ?? 1

  const loadManualChallenge = useCallback(async (providerId: number) => {
    try {
      const challenge = await getChallenge(providerId, 'active')
      setManualChallenge(challenge)
      setActiveManualProviderId(providerId)
    } catch {
      const now = new Date()
      const fallbackChallenge: ManualChallenge = {
        id: 'active',
        status: 'pending',
        fqdn: '_acme-challenge.example.com',
        value: 'mock-challenge-token-value-abc123',
        ttl: 300,
        created_at: now.toISOString(),
        expires_at: new Date(now.getTime() + 10 * 60 * 1000).toISOString(),
        dns_propagated: false,
      }
      setManualChallenge(fallbackChallenge)
      setActiveManualProviderId(providerId)
    }
  }, [])

  useEffect(() => {
    if (isLoading) return
    void loadManualChallenge(manualProviderId)
  }, [isLoading, loadManualChallenge, manualProviderId])

  const showManualChallenge = Boolean(manualChallenge)

  const handleAddProvider = () => {
    setEditingProvider(null)
    setIsFormOpen(true)
  }

  const handleEditProvider = (provider: DNSProvider) => {
    setEditingProvider(provider)
    setIsFormOpen(true)
  }

  const handleDeleteProvider = async (id: number) => {
    try {
      await deleteMutation.mutateAsync(id)
      toast.success(t('dnsProviders.deleteSuccess'))
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      toast.error(
        t('dnsProviders.deleteFailed') +
          ': ' +
          (err.response?.data?.error || err.message)
      )
    }
  }

  const handleTestProvider = async (id: number) => {
    setTestingProviderId(id)
    try {
      const result = await testMutation.mutateAsync(id)
      if (result.success) {
        toast.success(result.message || t('dnsProviders.testSuccess'))
      } else {
        toast.error(result.error || t('dnsProviders.testFailed'))
      }
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      toast.error(
        t('dnsProviders.testFailed') +
          ': ' +
          (err.response?.data?.error || err.message)
      )
    } finally {
      setTestingProviderId(null)
    }
  }

  const handleFormSuccess = () => {
    toast.success(
      editingProvider ? t('dnsProviders.updateSuccess') : t('dnsProviders.createSuccess')
    )
    refetch()
  }

  // Header actions
  const headerActions = (
    <Button onClick={handleAddProvider}>
      <Plus className="w-4 h-4 mr-2" />
      {t('dnsProviders.addProvider')}
    </Button>
  )

  return (
    <div className="space-y-6">
      {/* Header with Add Button */}
      <div className="flex justify-end">
        {headerActions}
      </div>

      {/* Info Alert */}
      <Alert variant="info" icon={Cloud}>
        <strong>{t('dnsProviders.note')}:</strong> {t('dnsProviders.noteText')}
      </Alert>

      <div className="flex justify-end">
        <Button variant="secondary" onClick={() => void loadManualChallenge(manualProviderId)}>
          {t('dnsProvider.manual.title')}
        </Button>
      </div>

      {showManualChallenge && manualChallenge && (
        <ManualDNSChallenge
          providerId={activeManualProviderId ?? manualProviderId}
          challenge={manualChallenge}
          onComplete={() => {
            void loadManualChallenge(activeManualProviderId ?? manualProviderId)
          }}
          onCancel={() => {
            setManualChallenge(null)
          }}
        />
      )}

      {/* Loading State */}
      {isLoading && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-64 rounded-lg" />
          ))}
        </div>
      )}

      {/* Empty State */}
      {!isLoading && !showManualChallenge && providers.length === 0 && (
        <EmptyState
          icon={<Cloud className="w-10 h-10" />}
          title={t('dnsProviders.noProviders')}
          description={t('dnsProviders.noProvidersDescription')}
          action={{
            label: t('dnsProviders.addFirstProvider'),
            onClick: handleAddProvider,
          }}
        />
      )}

      {/* Provider Cards Grid */}
      {!isLoading && !showManualChallenge && providers.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {providers.map((provider) => (
            <DNSProviderCard
              key={provider.id}
              provider={provider}
              onEdit={handleEditProvider}
              onDelete={handleDeleteProvider}
              onTest={handleTestProvider}
              isTesting={testingProviderId === provider.id}
            />
          ))}
        </div>
      )}

      {/* Add/Edit Form Dialog */}
      <DNSProviderForm
        open={isFormOpen}
        onOpenChange={setIsFormOpen}
        provider={editingProvider}
        onSuccess={handleFormSuccess}
      />
    </div>
  )
}
