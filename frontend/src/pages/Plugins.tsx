import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, Package, AlertCircle, CheckCircle, XCircle, Info } from 'lucide-react'
import {
  Button,
  Badge,
  Alert,
  EmptyState,
  Skeleton,
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
  Switch,
  Card,
} from '../components/ui'
import {
  usePlugins,
  useEnablePlugin,
  useDisablePlugin,
  useReloadPlugins,
  type PluginInfo,
} from '../hooks/usePlugins'
import { toast } from '../utils/toast'

export default function Plugins() {
  const { t } = useTranslation()
  const { data: plugins = [], isLoading, refetch } = usePlugins()
  const enableMutation = useEnablePlugin()
  const disableMutation = useDisablePlugin()
  const reloadMutation = useReloadPlugins()

  const [selectedPlugin, setSelectedPlugin] = useState<PluginInfo | null>(null)
  const [metadataModalOpen, setMetadataModalOpen] = useState(false)

  const handleTogglePlugin = async (plugin: PluginInfo) => {
    if (plugin.is_built_in) {
      toast.error(t('plugins.cannotDisableBuiltIn', 'Built-in plugins cannot be disabled'))
      return
    }

    try {
      if (plugin.enabled) {
        await disableMutation.mutateAsync(plugin.id)
        toast.success(t('plugins.disableSuccess', 'Plugin disabled successfully'))
      } else {
        await enableMutation.mutateAsync(plugin.id)
        toast.success(t('plugins.enableSuccess', 'Plugin enabled successfully'))
      }
      refetch()
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      const message =
        err.response?.data?.error || err.message || t('plugins.toggleFailed', 'Failed to toggle plugin')
      toast.error(message)
    }
  }

  const handleReloadPlugins = async () => {
    try {
      const result = await reloadMutation.mutateAsync()
      toast.success(
        t('plugins.reloadSuccess', 'Plugins reloaded: {{count}} loaded', { count: result.count })
      )
      refetch()
    } catch (error: unknown) {
      const err = error as { response?: { data?: { error?: string } }; message?: string }
      const message =
        err.response?.data?.error || err.message || t('plugins.reloadFailed', 'Failed to reload plugins')
      toast.error(message)
    }
  }

  const handleViewMetadata = (plugin: PluginInfo) => {
    setSelectedPlugin(plugin)
    setMetadataModalOpen(true)
  }

  const getStatusBadge = (plugin: PluginInfo) => {
    if (!plugin.enabled) {
      return (
        <Badge variant="secondary">
          <XCircle className="w-3 h-3 mr-1" />
          {t('plugins.disabled', 'Disabled')}
        </Badge>
      )
    }

    switch (plugin.status) {
      case 'loaded':
        return (
          <Badge variant="success">
            <CheckCircle className="w-3 h-3 mr-1" />
            {t('plugins.loaded', 'Loaded')}
          </Badge>
        )
      case 'error':
        return (
          <Badge variant="error">
            <AlertCircle className="w-3 h-3 mr-1" />
            {t('plugins.error', 'Error')}
          </Badge>
        )
      case 'pending':
        return (
          <Badge variant="warning">
            <Info className="w-3 h-3 mr-1" />
            {t('plugins.pending', 'Pending')}
          </Badge>
        )
      default:
        return null
    }
  }

  // Group plugins by type
  const builtInPlugins = plugins.filter((p) => p.is_built_in)
  const externalPlugins = plugins.filter((p) => !p.is_built_in)

  // Header actions
  const headerActions = (
    <Button onClick={handleReloadPlugins} variant="secondary" isLoading={reloadMutation.isPending}>
      <RefreshCw className="w-4 h-4 mr-2" />
      {t('plugins.reloadPlugins', 'Reload Plugins')}
    </Button>
  )

  return (
    <div className="space-y-6">
      {/* Header with Reload Button */}
      <div className="flex justify-end">
        {headerActions}
      </div>

      {/* Info Alert */}
      <Alert variant="info" icon={Package}>
        <strong>{t('plugins.note', 'Note')}:</strong>{' '}
        {t(
          'plugins.noteText',
          'External plugins extend Charon with custom DNS providers. Only install plugins from trusted sources.'
        )}
      </Alert>

      {/* Loading State */}
      {isLoading && (
        <div className="space-y-4">
          {[1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-32 rounded-lg" />
          ))}
        </div>
      )}

      {/* Empty State */}
      {!isLoading && plugins.length === 0 && (
        <EmptyState
          icon={<Package className="w-10 h-10" />}
          title={t('plugins.noPlugins', 'No Plugins Found')}
          description={t(
            'plugins.noPluginsDescription',
            'No DNS provider plugins are currently installed.'
          )}
        />
      )}

      {/* Built-in Plugins Section */}
      {!isLoading && builtInPlugins.length > 0 && (
        <div className="space-y-4">
          <h2 className="text-lg font-semibold text-content-primary">
            {t('plugins.builtInPlugins', 'Built-in Providers')}
          </h2>
          <div className="grid grid-cols-1 gap-4">
            {builtInPlugins.map((plugin) => (
              <Card key={plugin.type} className="p-4">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3">
                      <Package className="w-5 h-5 text-content-secondary flex-shrink-0" />
                      <div className="flex-1 min-w-0">
                        <h3 className="text-base font-medium text-content-primary truncate">
                          {plugin.name}
                        </h3>
                        <p className="text-sm text-content-secondary mt-0.5">
                          {plugin.type}
                          {plugin.version && (
                            <span className="ml-2 text-xs text-content-tertiary">
                              v{plugin.version}
                            </span>
                          )}
                        </p>
                        {plugin.description && (
                          <p className="text-sm text-content-tertiary mt-2">{plugin.description}</p>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 ml-4">
                    {getStatusBadge(plugin)}
                    {plugin.documentation_url && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => window.open(plugin.documentation_url, '_blank')}
                      >
                        {t('plugins.docs', 'Docs')}
                      </Button>
                    )}
                    <Button variant="outline" size="sm" onClick={() => handleViewMetadata(plugin)}>
                      {t('plugins.details', 'Details')}
                    </Button>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </div>
      )}

      {/* External Plugins Section */}
      {!isLoading && externalPlugins.length > 0 && (
        <div className="space-y-4">
          <h2 className="text-lg font-semibold text-content-primary">
            {t('plugins.externalPlugins', 'External Plugins')}
          </h2>
          <div className="grid grid-cols-1 gap-4">
            {externalPlugins.map((plugin) => (
              <Card key={plugin.id} className="p-4">
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3">
                      <Package className="w-5 h-5 text-brand-500 flex-shrink-0" />
                      <div className="flex-1 min-w-0">
                        <h3 className="text-base font-medium text-content-primary truncate">
                          {plugin.name}
                        </h3>
                        <p className="text-sm text-content-secondary mt-0.5">
                          {plugin.type}
                          {plugin.version && (
                            <span className="ml-2 text-xs text-content-tertiary">
                              v{plugin.version}
                            </span>
                          )}
                          {plugin.author && (
                            <span className="ml-2 text-xs text-content-tertiary">
                              by {plugin.author}
                            </span>
                          )}
                        </p>
                        {plugin.description && (
                          <p className="text-sm text-content-tertiary mt-2">{plugin.description}</p>
                        )}
                        {plugin.error && (
                          <Alert variant="error" className="mt-2">
                            <p className="text-sm">{plugin.error}</p>
                          </Alert>
                        )}
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-3 ml-4">
                    {getStatusBadge(plugin)}
                    <Switch
                      checked={plugin.enabled}
                      onCheckedChange={() => handleTogglePlugin(plugin)}
                      disabled={enableMutation.isPending || disableMutation.isPending}
                    />
                    {plugin.documentation_url && (
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => window.open(plugin.documentation_url, '_blank')}
                      >
                        {t('plugins.docs', 'Docs')}
                      </Button>
                    )}
                    <Button variant="outline" size="sm" onClick={() => handleViewMetadata(plugin)}>
                      {t('plugins.details', 'Details')}
                    </Button>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </div>
      )}

      {/* Metadata Modal */}
      <Dialog open={metadataModalOpen} onOpenChange={setMetadataModalOpen}>
        <DialogContent className="max-w-2xl">
          <DialogHeader>
            <DialogTitle>
              {t('plugins.pluginDetails', 'Plugin Details')}: {selectedPlugin?.name}
            </DialogTitle>
          </DialogHeader>
          {selectedPlugin && (
            <div className="space-y-4 py-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <p className="text-sm font-medium text-content-secondary">
                    {t('plugins.type', 'Type')}
                  </p>
                  <p className="text-base text-content-primary">{selectedPlugin.type}</p>
                </div>
                <div>
                  <p className="text-sm font-medium text-content-secondary">
                    {t('plugins.status', 'Status')}
                  </p>
                  <div className="mt-1">{getStatusBadge(selectedPlugin)}</div>
                </div>
                {selectedPlugin.version && (
                  <div>
                    <p className="text-sm font-medium text-content-secondary">
                      {t('plugins.version', 'Version')}
                    </p>
                    <p className="text-base text-content-primary">{selectedPlugin.version}</p>
                  </div>
                )}
                {selectedPlugin.author && (
                  <div>
                    <p className="text-sm font-medium text-content-secondary">
                      {t('plugins.author', 'Author')}
                    </p>
                    <p className="text-base text-content-primary">{selectedPlugin.author}</p>
                  </div>
                )}
                <div>
                  <p className="text-sm font-medium text-content-secondary">
                    {t('plugins.pluginType', 'Plugin Type')}
                  </p>
                  <p className="text-base text-content-primary">
                    {selectedPlugin.is_built_in
                      ? t('plugins.builtIn', 'Built-in')
                      : t('plugins.external', 'External')}
                  </p>
                </div>
                {selectedPlugin.loaded_at && (
                  <div>
                    <p className="text-sm font-medium text-content-secondary">
                      {t('plugins.loadedAt', 'Loaded At')}
                    </p>
                    <p className="text-base text-content-primary">
                      {new Date(selectedPlugin.loaded_at).toLocaleString()}
                    </p>
                  </div>
                )}
              </div>
              {selectedPlugin.description && (
                <div>
                  <p className="text-sm font-medium text-content-secondary mb-2">
                    {t('plugins.description', 'Description')}
                  </p>
                  <p className="text-sm text-content-primary">{selectedPlugin.description}</p>
                </div>
              )}
              {selectedPlugin.documentation_url && (
                <div>
                  <p className="text-sm font-medium text-content-secondary mb-2">
                    {t('plugins.documentation', 'Documentation')}
                  </p>
                  <a
                    href={selectedPlugin.documentation_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-sm text-brand-500 hover:text-brand-600 underline"
                  >
                    {selectedPlugin.documentation_url}
                  </a>
                </div>
              )}
              {selectedPlugin.error && (
                <div>
                  <p className="text-sm font-medium text-content-secondary mb-2">
                    {t('plugins.errorDetails', 'Error Details')}
                  </p>
                  <Alert variant="error">
                    <p className="text-sm font-mono">{selectedPlugin.error}</p>
                  </Alert>
                </div>
              )}
            </div>
          )}
          <DialogFooter>
            <Button onClick={() => setMetadataModalOpen(false)}>
              {t('common.close', 'Close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
