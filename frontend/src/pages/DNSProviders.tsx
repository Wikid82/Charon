import { Plus, Cloud } from 'lucide-react'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getChallenge, type ManualChallenge } from '../api/manualChallenge'
import { ManualDNSChallenge } from '../components/dns-providers'
import DNSProviderCard from '../components/DNSProviderCard'
import DNSProviderForm from '../components/DNSProviderForm'
import { Button, Alert, EmptyState, Skeleton } from '../components/ui'
import { useDNSProviders, useDNSProviderMutations, type DNSProvider } from '../hooks/useDNSProviders'
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
  const [isManualChallengeOpen, setIsManualChallengeOpen] = useState(false)

  const manualProviderId = providers.find((provider) => provider.provider_type === 'manual')?.id ?? null

  const loadManualChallenge = useCallback(async (providerId: number): Promise<boolean> => {
    try {
      const challenge = await getChallenge(providerId, 'active')
      setManualChallenge(challenge)
      setActiveManualProviderId(providerId)
      return true
    } catch {
      setManualChallenge(null)
      setActiveManualProviderId(providerId)
      return false
    }
  }, [])

  const manualChallengeProviderId = activeManualProviderId ?? manualProviderId
  const showManualChallenge =
    isManualChallengeOpen && Boolean(manualChallenge) && manualChallengeProviderId !== null

  const handleManualChallengeClick = async () => {
    if (manualProviderId === null) {
      toast.error(t('dnsProviders.noProviders'))
      return
    }

    const hasChallenge = await loadManualChallenge(manualProviderId)

    if (!hasChallenge) {
      toast.error(t('dnsProvider.manual.challengeNotFound'))
      setIsManualChallengeOpen(false)
      return
    }

    setIsManualChallengeOpen(true)
  }

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
        <Button variant="secondary" onClick={() => void handleManualChallengeClick()}>
          {t('dnsProvider.manual.title')}
        </Button>
      </div>

      {showManualChallenge && manualChallenge && manualChallengeProviderId !== null && (
        <ManualDNSChallenge
          providerId={manualChallengeProviderId}
          challenge={manualChallenge}
          onComplete={() => {
            const providerId = activeManualProviderId ?? manualProviderId
            if (providerId === null) {
              setIsManualChallengeOpen(false)
              return
            }

            void loadManualChallenge(providerId).then((hasChallenge) => {
              setIsManualChallengeOpen(hasChallenge)
            })
          }}
          onCancel={() => {
            setManualChallenge(null)
            setIsManualChallengeOpen(false)
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
