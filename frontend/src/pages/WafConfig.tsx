import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Shield, Plus, Pencil, Trash2, ExternalLink, FileCode2, Sparkles } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Input } from '../components/ui/Input'
import { useRuleSets, useUpsertRuleSet, useDeleteRuleSet } from '../hooks/useSecurity'
import type { SecurityRuleSet, UpsertRuleSetPayload } from '../api/security'
import { ConfigReloadOverlay } from '../components/LoadingStates'

/**
 * WAF Rule Presets for common security configurations
 */
const WAF_PRESETS = [
  {
    name: 'OWASP Core Rule Set',
    url: 'https://github.com/coreruleset/coreruleset/archive/refs/tags/v3.3.5.tar.gz',
    content: '',
    description: 'Industry standard protection against OWASP Top 10 vulnerabilities.',
  },
  {
    name: 'Basic SQL Injection Protection',
    url: '',
    content: `SecRule ARGS "@detectSQLi" "id:1001,phase:1,deny,status:403,msg:'SQLi Detected'"
SecRule REQUEST_BODY "@detectSQLi" "id:1002,phase:2,deny,status:403,msg:'SQLi in Body'"
SecRule REQUEST_COOKIES "@detectSQLi" "id:1003,phase:1,deny,status:403,msg:'SQLi in Cookies'"`,
    description: 'Simple rules to block common SQL injection patterns.',
  },
  {
    name: 'Basic XSS Protection',
    url: '',
    content: `SecRule ARGS "@detectXSS" "id:2001,phase:1,deny,status:403,msg:'XSS Detected'"
SecRule REQUEST_BODY "@detectXSS" "id:2002,phase:2,deny,status:403,msg:'XSS in Body'"`,
    description: 'Rules to block common Cross-Site Scripting (XSS) attacks.',
  },
  {
    name: 'Common Bad Bots',
    url: '',
    content: `SecRule REQUEST_HEADERS:User-Agent "@rx (?i)(curl|curl|python|scrapy|httpclient|libwww|nikto|sqlmap)" "id:3001,phase:1,deny,status:403,msg:'Bad Bot Detected'"
SecRule REQUEST_HEADERS:User-Agent "@streq -" "id:3002,phase:1,deny,status:403,msg:'Empty User-Agent'"`,
    description: 'Block known malicious bots and scanners.',
  },
] as const

/**
 * Confirmation dialog for destructive actions
 */
function ConfirmDialog({
  isOpen,
  title,
  message,
  confirmLabel,
  cancelLabel,
  onConfirm,
  onCancel,
  isLoading,
  deletingLabel,
}: {
  isOpen: boolean
  title: string
  message: string
  confirmLabel: string
  cancelLabel: string
  onConfirm: () => void
  onCancel: () => void
  isLoading?: boolean
  deletingLabel: string
}) {
  if (!isOpen) return null

  return (
    <div
      className="fixed inset-0 bg-black/50 flex items-center justify-center z-50"
      onClick={onCancel}
      data-testid="confirm-dialog-backdrop"
    >
      <div
        className="bg-dark-card border border-gray-800 rounded-lg p-6 max-w-md w-full mx-4"
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="text-xl font-bold text-white mb-2">{title}</h2>
        <p className="text-gray-400 mb-6">{message}</p>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onCancel} disabled={isLoading}>
            {cancelLabel}
          </Button>
          <Button
            variant="danger"
            onClick={onConfirm}
            disabled={isLoading}
            data-testid="confirm-delete-btn"
          >
            {isLoading ? deletingLabel : confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  )
}

/**
 * Form for creating/editing a WAF rule set
 */
function RuleSetForm({
  initialData,
  onSubmit,
  onCancel,
  isLoading,
  t,
}: {
  initialData?: SecurityRuleSet
  onSubmit: (data: UpsertRuleSetPayload) => void
  onCancel: () => void
  isLoading?: boolean
  t: (key: string) => string
}) {
  const [name, setName] = useState(initialData?.name || '')
  const [sourceUrl, setSourceUrl] = useState(initialData?.source_url || '')
  const [content, setContent] = useState(initialData?.content || '')
  const [mode, setMode] = useState<'blocking' | 'detection'>(
    initialData?.mode === 'detection' ? 'detection' : 'blocking'
  )
  const [selectedPreset, setSelectedPreset] = useState('')

  const handlePresetChange = (presetName: string) => {
    setSelectedPreset(presetName)
    if (presetName === '') return

    const preset = WAF_PRESETS.find((p) => p.name === presetName)
    if (preset) {
      setName(preset.name)
      setSourceUrl(preset.url)
      setContent(preset.content)
    }
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSubmit({
      id: initialData?.id,
      name: name.trim(),
      source_url: sourceUrl.trim() || undefined,
      content: content.trim() || undefined,
      mode,
    })
  }

  const isValid = name.trim().length > 0 && (content.trim().length > 0 || sourceUrl.trim().length > 0)

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      {/* Presets Dropdown - only show when creating new */}
      {!initialData && (
        <div>
          <label className="block text-sm font-medium text-gray-300 mb-1.5">
            <Sparkles className="inline h-4 w-4 mr-1 text-yellow-400" />
            {t('wafConfig.quickStartPreset')}
          </label>
          <select
            value={selectedPreset}
            onChange={(e) => handlePresetChange(e.target.value)}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            data-testid="preset-select"
          >
            <option value="">{t('wafConfig.choosePreset')}</option>
            {WAF_PRESETS.map((preset) => (
              <option key={preset.name} value={preset.name}>
                {preset.name}
              </option>
            ))}
          </select>
          {selectedPreset && (
            <p className="mt-1 text-xs text-gray-500">
              {WAF_PRESETS.find((p) => p.name === selectedPreset)?.description}
            </p>
          )}
        </div>
      )}

      <Input
        label={t('wafConfig.ruleSetName')}
        value={name}
        onChange={(e) => setName(e.target.value)}
        placeholder={t('wafConfig.ruleSetNamePlaceholder')}
        required
        data-testid="ruleset-name-input"
      />

      <div>
        <label className="block text-sm font-medium text-gray-300 mb-1.5">{t('wafConfig.mode')}</label>
        <div className="flex gap-4">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              name="mode"
              value="blocking"
              checked={mode === 'blocking'}
              onChange={() => setMode('blocking')}
              className="text-blue-600 focus:ring-blue-500"
              data-testid="mode-blocking"
            />
            <span className="text-sm text-gray-300">{t('wafConfig.blocking')}</span>
          </label>
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="radio"
              name="mode"
              value="detection"
              checked={mode === 'detection'}
              onChange={() => setMode('detection')}
              className="text-blue-600 focus:ring-blue-500"
              data-testid="mode-detection"
            />
            <span className="text-sm text-gray-300">{t('wafConfig.detectionOnly')}</span>
          </label>
        </div>
        <p className="mt-1 text-xs text-gray-500">
          {mode === 'blocking'
            ? t('wafConfig.blockingDescription')
            : t('wafConfig.detectionDescription')}
        </p>
      </div>

      <Input
        label={t('wafConfig.sourceUrl')}
        value={sourceUrl}
        onChange={(e) => setSourceUrl(e.target.value)}
        placeholder="https://example.com/rules.conf"
        helperText={t('wafConfig.sourceUrlHelper')}
        data-testid="ruleset-url-input"
      />

      <div>
        <label className="block text-sm font-medium text-gray-300 mb-1.5">
          {t('wafConfig.ruleContent')} {!sourceUrl && <span className="text-red-400">*</span>}
        </label>
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder={`SecRule REQUEST_URI "@contains /admin" "id:1000,phase:1,deny,status:403"`}
          rows={10}
          className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white font-mono text-sm placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          data-testid="ruleset-content-input"
        />
        <p className="mt-1 text-xs text-gray-500">
          {t('wafConfig.ruleContentHelper')}
        </p>
      </div>

      <div className="flex justify-end gap-2 pt-4">
        <Button type="button" variant="secondary" onClick={onCancel}>
          {t('common.cancel')}
        </Button>
        <Button type="submit" disabled={!isValid || isLoading} isLoading={isLoading}>
          {initialData ? t('wafConfig.updateRuleSet') : t('wafConfig.createRuleSet')}
        </Button>
      </div>
    </form>
  )
}

/**
 * WAF Configuration Page - Manage Coraza rule sets
 */
export default function WafConfig() {
  const { t } = useTranslation()
  const { data: ruleSets, isLoading, error } = useRuleSets()
  const upsertMutation = useUpsertRuleSet()
  const deleteMutation = useDeleteRuleSet()

  const [showCreateForm, setShowCreateForm] = useState(false)
  const [editingRuleSet, setEditingRuleSet] = useState<SecurityRuleSet | null>(null)
  const [deleteConfirm, setDeleteConfirm] = useState<SecurityRuleSet | null>(null)

  // Determine if any security operation is in progress
  const isApplyingConfig = upsertMutation.isPending || deleteMutation.isPending

  // Determine contextual message based on operation
  const getMessage = () => {
    if (upsertMutation.isPending) {
      return editingRuleSet
        ? { message: t('wafConfig.cerberusAwakens'), submessage: t('wafConfig.guardianStandsWatch') }
        : { message: t('wafConfig.forgingDefenses'), submessage: t('wafConfig.rulesInscribing') }
    }
    if (deleteMutation.isPending) {
      return { message: t('wafConfig.loweringBarrier'), submessage: t('wafConfig.defenseLayerRemoved') }
    }
    return { message: t('wafConfig.cerberusAwakens'), submessage: t('wafConfig.guardianStandsWatch') }
  }

  const { message, submessage } = getMessage()

  const handleCreate = (data: UpsertRuleSetPayload) => {
    upsertMutation.mutate(data, {
      onSuccess: () => setShowCreateForm(false),
    })
  }

  const handleUpdate = (data: UpsertRuleSetPayload) => {
    upsertMutation.mutate(data, {
      onSuccess: () => setEditingRuleSet(null),
    })
  }

  const handleDelete = () => {
    if (!deleteConfirm) return
    deleteMutation.mutate(deleteConfirm.id, {
      onSuccess: () => {
        setDeleteConfirm(null)
        setEditingRuleSet(null)
      },
    })
  }

  if (isLoading) {
    return (
      <div className="p-8 text-center text-white" data-testid="waf-loading">
        {t('wafConfig.loadingConfiguration')}
      </div>
    )
  }

  if (error) {
    return (
      <div className="p-8 text-center text-red-400" data-testid="waf-error">
        {t('wafConfig.failedToLoad')}: {error instanceof Error ? error.message : t('common.unknownError')}
      </div>
    )
  }

  const ruleSetList = ruleSets?.rulesets || []

  return (
    <>
      {isApplyingConfig && (
        <ConfigReloadOverlay
          message={message}
          submessage={submessage}
          type="cerberus"
        />
      )}
      <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-2">
            <Shield className="w-7 h-7 text-blue-400" />
            {t('wafConfig.title')}
          </h1>
          <p className="text-gray-400 mt-1">
            {t('wafConfig.description')}
          </p>
        </div>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            size="sm"
            onClick={() =>
              window.open('https://coraza.io/docs/seclang/directives/', '_blank')
            }
          >
            <ExternalLink className="h-4 w-4 mr-2" />
            {t('wafConfig.ruleSyntax')}
          </Button>
          <Button onClick={() => setShowCreateForm(true)} data-testid="create-ruleset-btn">
            <Plus className="h-4 w-4 mr-2" />
            {t('wafConfig.addRuleSet')}
          </Button>
        </div>
      </div>

      {/* Info Banner */}
      <div className="bg-blue-900/20 border border-blue-800/50 rounded-lg p-4">
        <div className="flex items-start gap-3">
          <FileCode2 className="h-5 w-5 text-blue-400 shrink-0 mt-0.5" />
          <div>
            <h3 className="text-sm font-semibold text-blue-300 mb-1">
              {t('wafConfig.aboutTitle')}
            </h3>
            <p className="text-sm text-blue-200/90">
              {t('wafConfig.aboutDescription')}
            </p>
          </div>
        </div>
      </div>

      {/* Create Form */}
      {showCreateForm && (
        <div className="bg-dark-card border border-gray-800 rounded-lg p-6">
          <h2 className="text-xl font-bold text-white mb-4">{t('wafConfig.createRuleSet')}</h2>
          <RuleSetForm
            onSubmit={handleCreate}
            onCancel={() => setShowCreateForm(false)}
            isLoading={upsertMutation.isPending}
            t={t}
          />
        </div>
      )}

      {/* Edit Form */}
      {editingRuleSet && (
        <div className="bg-dark-card border border-gray-800 rounded-lg p-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-bold text-white">{t('wafConfig.editRuleSet')}</h2>
            <Button
              variant="danger"
              size="sm"
              onClick={() => setDeleteConfirm(editingRuleSet)}
            >
              <Trash2 className="h-4 w-4 mr-2" />
              {t('common.delete')}
            </Button>
          </div>
          <RuleSetForm
            initialData={editingRuleSet}
            onSubmit={handleUpdate}
            onCancel={() => setEditingRuleSet(null)}
            isLoading={upsertMutation.isPending}
            t={t}
          />
        </div>
      )}

      {/* Delete Confirmation */}
      <ConfirmDialog
        isOpen={deleteConfirm !== null}
        title={t('wafConfig.deleteRuleSet')}
        message={t('wafConfig.deleteConfirmation', { name: deleteConfirm?.name })}
        confirmLabel={t('common.delete')}
        cancelLabel={t('common.cancel')}
        deletingLabel={t('common.deleting')}
        onConfirm={handleDelete}
        onCancel={() => setDeleteConfirm(null)}
        isLoading={deleteMutation.isPending}
      />

      {/* Empty State */}
      {ruleSetList.length === 0 && !showCreateForm && !editingRuleSet && (
        <div
          className="bg-dark-card border border-gray-800 rounded-lg p-12 text-center"
          data-testid="waf-empty-state"
        >
          <div className="text-gray-500 mb-4 text-4xl">🛡️</div>
          <h3 className="text-lg font-semibold text-white mb-2">{t('wafConfig.noRuleSets')}</h3>
          <p className="text-gray-400 mb-4">
            {t('wafConfig.noRuleSetsDescription')}
          </p>
          <Button onClick={() => setShowCreateForm(true)}>
            <Plus className="h-4 w-4 mr-2" />
            {t('wafConfig.createRuleSet')}
          </Button>
        </div>
      )}

      {/* Rule Sets Table */}
      {ruleSetList.length > 0 && !showCreateForm && !editingRuleSet && (
        <div className="bg-dark-card border border-gray-800 rounded-lg overflow-hidden">
          <table className="w-full" data-testid="rulesets-table">
            <thead className="bg-gray-900/50 border-b border-gray-800">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">
                  {t('common.name')}
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">
                  {t('wafConfig.mode')}
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">
                  {t('wafConfig.source')}
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-400 uppercase">
                  {t('wafConfig.lastUpdated')}
                </th>
                <th className="px-6 py-3 text-right text-xs font-medium text-gray-400 uppercase">
                  {t('common.actions')}
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-800">
              {ruleSetList.map((rs) => (
                <tr key={rs.id} className="hover:bg-gray-900/30">
                  <td className="px-6 py-4">
                    <p className="font-medium text-white">{rs.name}</p>
                    {rs.content && (
                      <p className="text-xs text-gray-500 mt-1">
                        {t('wafConfig.ruleCount', { count: rs.content.split('\n').filter((l) => l.trim()).length })}
                      </p>
                    )}
                  </td>
                  <td className="px-6 py-4">
                    <span
                      className={`px-2 py-1 text-xs rounded ${
                        rs.mode === 'blocking'
                          ? 'bg-red-900/30 text-red-300'
                          : 'bg-yellow-900/30 text-yellow-300'
                      }`}
                    >
                      {rs.mode === 'blocking' ? t('wafConfig.blocking') : t('wafConfig.detection')}
                    </span>
                  </td>
                  <td className="px-6 py-4">
                    {rs.source_url ? (
                      <a
                        href={rs.source_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-sm text-blue-400 hover:underline flex items-center gap-1"
                      >
                        {t('wafConfig.url')}
                        <ExternalLink className="h-3 w-3" />
                      </a>
                    ) : (
                      <span className="text-sm text-gray-500">{t('wafConfig.inline')}</span>
                    )}
                  </td>
                  <td className="px-6 py-4 text-sm text-gray-400">
                    {rs.last_updated
                      ? new Date(rs.last_updated).toLocaleDateString()
                      : '-'}
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex justify-end gap-2">
                      <button
                        onClick={() => setEditingRuleSet(rs)}
                        className="text-gray-400 hover:text-blue-400"
                        title={t('common.edit')}
                        data-testid={`edit-ruleset-${rs.id}`}
                      >
                        <Pencil className="h-4 w-4" />
                      </button>
                      <button
                        onClick={() => setDeleteConfirm(rs)}
                        className="text-gray-400 hover:text-red-400"
                        title={t('common.delete')}
                        data-testid={`delete-ruleset-${rs.id}`}
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      </div>
    </>
  )
}
