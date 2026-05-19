import { Plus } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { type InstallSnippets } from '../api/orthrus'
import { OrthrusAgentManager } from '../components/hecate/OrthrusAgentManager'
import { OrthrusInstallWizard } from '../components/hecate/OrthrusInstallWizard'
import { PageShell } from '../components/layout/PageShell'
import {
  Button,
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Label,
} from '../components/ui'
import { useAgentList, useProvisionAgent, useOrthrus } from '../hooks/useOrthrus'

export default function HecateAgent() {
  const { t } = useTranslation()

  const { data: agents = [] } = useAgentList()
  const { mutateAsync: doProvision } = useProvisionAgent()
  const { getInstallSnippets } = useOrthrus()

  const [provisionOpen, setProvisionOpen] = useState(false)
  const [provisionName, setProvisionName] = useState('')
  const [provisionError, setProvisionError] = useState<string | null>(null)
  const [isProvisioning, setIsProvisioning] = useState(false)
  const [wizardData, setWizardData] = useState<{
    agentName: string
    agentUUID: string
    authKey: string
    snippets: InstallSnippets
  } | null>(null)

  const handleProvision = async () => {
    if (!provisionName.trim()) return
    setIsProvisioning(true)
    setProvisionError(null)
    try {
      const result = await doProvision({ name: provisionName.trim() })
      const snippets = await getInstallSnippets(result.agent.uuid)
      setWizardData({
        agentName: result.agent.name,
        agentUUID: result.agent.uuid,
        authKey: result.auth_key,
        snippets,
      })
      setProvisionOpen(false)
      setProvisionName('')
    } catch (err) {
      setProvisionError(err instanceof Error ? err.message : 'Failed to provision agent')
    } finally {
      setIsProvisioning(false)
    }
  }

  return (
    <PageShell
      title={t('hecate.agent.title')}
      description={t('hecate.agent.description')}
    >
      <section aria-labelledby="orthrus-section-heading">
        <div className="flex items-center justify-between mb-4">
          <h2
            id="orthrus-section-heading"
            className="text-lg font-semibold text-content-primary"
          >
            {t('hecate.page.orthrusSection')}
          </h2>
          <Button variant="secondary" onClick={() => setProvisionOpen(true)}>
            <Plus className="w-4 h-4 mr-2" />
            {t('hecate.page.provisionAgent')}
          </Button>
        </div>
        <OrthrusAgentManager agents={agents} />
      </section>

      <Dialog open={provisionOpen} onOpenChange={() => setProvisionOpen(false)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('hecate.page.provisionAgent')}</DialogTitle>
          </DialogHeader>
          <div className="py-4 space-y-4">
            <div>
              <Label htmlFor="provision-name">
                {t('common.name')}
                <span aria-hidden="true"> *</span>
              </Label>
              <input
                id="provision-name"
                type="text"
                required
                aria-required
                value={provisionName}
                onChange={e => setProvisionName(e.target.value)}
                className="w-full mt-1 bg-surface-subtle border border-border rounded-lg px-4 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
            {provisionError && (
              <p role="alert" className="text-sm text-error">
                {provisionError}
              </p>
            )}
          </div>
          <DialogFooter>
            <Button
              variant="secondary"
              onClick={() => setProvisionOpen(false)}
              disabled={isProvisioning}
            >
              {t('common.cancel')}
            </Button>
            <Button
              onClick={handleProvision}
              disabled={isProvisioning || !provisionName.trim()}
            >
              {isProvisioning ? t('common.loading') : t('hecate.page.provisionAgent')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {wizardData && (
        <OrthrusInstallWizard
          agentName={wizardData.agentName}
          agentUUID={wizardData.agentUUID}
          authKey={wizardData.authKey}
          snippets={wizardData.snippets}
          open
          onClose={() => setWizardData(null)}
        />
      )}
    </PageShell>
  )
}
