import { useState, type FC } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getProviders, createProvider, updateProvider, deleteProvider, testProvider, getTemplates, previewProvider, NotificationProvider, getExternalTemplates, previewExternalTemplate, ExternalTemplate, createExternalTemplate, updateExternalTemplate, deleteExternalTemplate, NotificationTemplate } from '../api/notifications';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Bell, Plus, Trash2, Edit2, Send, Check, X, Loader2 } from 'lucide-react';
import { useForm } from 'react-hook-form';

// supportsJSONTemplates returns true if the provider type can use JSON templates
const supportsJSONTemplates = (providerType: string | undefined): boolean => {
  if (!providerType) return false;
  switch (providerType.toLowerCase()) {
    case 'webhook':
    case 'discord':
    case 'slack':
    case 'gotify':
    case 'generic':
      return true;
    case 'telegram':
      return false; // Telegram uses URL parameters
    default:
      return false;
  }
};

const ProviderForm: FC<{
  initialData?: Partial<NotificationProvider>;
  onClose: () => void;
  onSubmit: (data: Partial<NotificationProvider>) => void;
}> = ({ initialData, onClose, onSubmit }) => {
  const { t } = useTranslation();
  const { register, handleSubmit, watch, setValue, formState: { errors } } = useForm({
    defaultValues: initialData || {
      type: 'discord',
      enabled: true,
      config: '',
      template: 'minimal',
      notify_proxy_hosts: true,
      notify_remote_servers: true,
      notify_domains: true,
      notify_certs: true,
      notify_uptime: true
    }
  });

  const [testStatus, setTestStatus] = useState<'idle' | 'success' | 'error'>('idle');
  const [previewContent, setPreviewContent] = useState<string | null>(null);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const testMutation = useMutation({
    mutationFn: testProvider,
    onSuccess: () => {
      setTestStatus('success');
      setTimeout(() => setTestStatus('idle'), 3000);
    },
    onError: () => {
      setTestStatus('error');
      setTimeout(() => setTestStatus('idle'), 3000);
    }
  });

  const handleTest = () => {
    const formData = watch();
    testMutation.mutate(formData as Partial<NotificationProvider>);
  };

  const handlePreview = async () => {
    const formData = watch();
    setPreviewContent(null);
    setPreviewError(null);
    try {
      // If using an external saved template (id), call previewExternalTemplate with template_id
      if (formData.template && typeof formData.template === 'string' && formData.template.length === 36) {
        const res = await previewExternalTemplate(formData.template, undefined, undefined);
        if (res.parsed) setPreviewContent(JSON.stringify(res.parsed, null, 2)); else setPreviewContent(res.rendered);
      } else {
        const res = await previewProvider(formData as Partial<NotificationProvider>);
        if (res.parsed) setPreviewContent(JSON.stringify(res.parsed, null, 2)); else setPreviewContent(res.rendered);
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setPreviewError(msg || 'Failed to generate preview');
    }
  };

  const type = watch('type');
  const { data: builtins } = useQuery({ queryKey: ['notificationTemplates'], queryFn: getTemplates });
  const { data: externalTemplates } = useQuery({ queryKey: ['externalTemplates'], queryFn: getExternalTemplates });
  const template = watch('template');

  const setTemplate = (templateStr: string, templateName?: string) => {
    // If templateName is provided, set template selection as well
    if (templateName) setValue('template', templateName);
    setValue('config', templateStr);
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.providerName')}</label>
        <input
          {...register('name', { required: t('errors.required') as string })}
          data-testid="provider-name"
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
        />
        {errors.name && <span className="text-red-500 text-xs">{errors.name.message as string}</span>}
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('common.type')}</label>
        <select
          {...register('type')}
          data-testid="provider-type"
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
        >
          <option value="discord">Discord</option>
          <option value="slack">Slack</option>
          <option value="gotify">Gotify</option>
          <option value="telegram">Telegram</option>
          <option value="generic">{t('notificationProviders.genericWebhook')}</option>
          <option value="webhook">{t('notificationProviders.customWebhook')}</option>
        </select>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.urlWebhook')}</label>
        <input
          {...register('url', { required: t('notificationProviders.urlRequired') as string })}
          data-testid="provider-url"
          placeholder="https://discord.com/api/webhooks/..."
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
        />
        {!supportsJSONTemplates(type) && (
          <p className="text-xs text-gray-500 mt-1">
            {t('notificationProviders.shoutrrrHelp')} <a href="https://containrrr.dev/shoutrrr/" target="_blank" rel="noreferrer" className="text-blue-500 hover:underline">{t('common.docs')}</a>.
          </p>
        )}
      </div>

      {supportsJSONTemplates(type) && (
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.jsonPayloadTemplate')}</label>
          <div className="flex gap-2 mb-2 mt-1">
            <Button type="button" size="sm" variant={template === 'minimal' ? 'primary' : 'secondary'} onClick={() => setTemplate('{"message": "{{.Message}}", "title": "{{.Title}}", "time": "{{.Time}}", "event": "{{.EventType}}"}', 'minimal')}>
              {t('notificationProviders.minimalTemplate')}
            </Button>
            <Button type="button" size="sm" variant={template === 'detailed' ? 'primary' : 'secondary'} onClick={() => setTemplate(`{"title": "{{.Title}}", "message": "{{.Message}}", "time": "{{.Time}}", "event": "{{.EventType}}", "host": "{{.HostName}}", "host_ip": "{{.HostIP}}", "service_count": {{.ServiceCount}}, "services": {{.Services}}}`, 'detailed')}>
              {t('notificationProviders.detailedTemplate')}
            </Button>
            <Button type="button" size="sm" variant={template === 'custom' ? 'primary' : 'secondary'} onClick={() => setValue('template', 'custom')}>
              {t('notificationProviders.customTemplate')}
            </Button>
          </div>
          <div className="mt-2">
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.template')}</label>
            <select {...register('template')} className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm">
              {/* Built-in template options */}
              {builtins?.map((t: NotificationTemplate) => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
              {/* External saved templates (id values are UUIDs) */}
              {externalTemplates?.map((t: ExternalTemplate) => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
          </div>
          <textarea
            {...register('config')}
            data-testid="provider-config"
            rows={8}
            className="mt-1 block w-full font-mono text-xs rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white"
            placeholder='{"text": "{{.Message}}"}'
          />
          <p className="text-xs text-gray-500 mt-1">
            {t('notificationProviders.availableVariables')}
          </p>
        </div>
      )}

      <div className="space-y-2 border-t border-gray-200 dark:border-gray-700 pt-4">
        <h4 className="text-sm font-medium text-gray-900 dark:text-white">{t('notificationProviders.notificationEvents')}</h4>
        <div className="grid grid-cols-2 gap-2">
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_proxy_hosts')} data-testid="notify-proxy-hosts" className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">{t('notificationProviders.proxyHosts')}</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_remote_servers')} data-testid="notify-remote-servers" className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">{t('notificationProviders.remoteServers')}</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_domains')} data-testid="notify-domains" className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">{t('notificationProviders.domainsNotify')}</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_certs')} data-testid="notify-certs" className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">{t('notificationProviders.certificates')}</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_uptime')} data-testid="notify-uptime" className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">{t('notificationProviders.uptime')}</label>
          </div>
        </div>
      </div>

      <div className="flex items-center">
        <input
          type="checkbox"
          {...register('enabled')}
          data-testid="provider-enabled"
          className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900 dark:text-gray-300">{t('common.enabled')}</label>
      </div>

      <div className="flex justify-end gap-2 pt-4">
        <Button variant="secondary" onClick={onClose}>{t('common.cancel')}</Button>
        <Button
          type="button"
          variant="secondary"
          onClick={handlePreview}
          disabled={testMutation.isPending}
          data-testid="provider-preview-btn"
          className="min-w-[80px]"
        >
          {t('notificationProviders.preview')}
        </Button>
        <Button
          type="button"
          variant="secondary"
          onClick={handleTest}
          disabled={testMutation.isPending}
          data-testid="provider-test-btn"
          className="min-w-[80px]"
        >
          {testMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin mx-auto" /> :
           testStatus === 'success' ? <Check className="w-4 h-4 text-green-500 mx-auto" /> :
           testStatus === 'error' ? <X className="w-4 h-4 text-red-500 mx-auto" /> :
           t('common.test')}
        </Button>
        <Button type="submit" data-testid="provider-save-btn">{t('common.save')}</Button>
      </div>
      {previewError && <div className="mt-2 text-sm text-red-600">{t('notificationProviders.previewError')}: {previewError}</div>}
      {previewContent && (
        <div className="mt-2">
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.previewResult')}</label>
          <pre className="mt-1 p-2 bg-gray-50 dark:bg-gray-800 rounded text-xs overflow-auto whitespace-pre-wrap">{previewContent}</pre>
        </div>
      )}
    </form>
  );
};

const TemplateForm: FC<{
  initialData?: Partial<ExternalTemplate>;
  onClose: () => void;
  onSubmit: (data: Partial<ExternalTemplate>) => void;
  }> = ({ initialData, onClose, onSubmit }) => {
    const { t } = useTranslation();
    const { register, handleSubmit, watch } = useForm({
    defaultValues: initialData || { template: 'custom', config: '' }
  });

  const [preview, setPreview] = useState<string | null>(null);
  const [previewErr, setPreviewErr] = useState<string | null>(null);

  const handlePreview = async () => {
    setPreview(null);
    setPreviewErr(null);
    const form = watch();
    try {
      const res = await previewExternalTemplate(undefined, form.config, { Title: 'Preview Title', Message: 'Preview Message', Time: new Date().toISOString(), EventType: 'preview' });
      if (res.parsed) setPreview(JSON.stringify(res.parsed, null, 2)); else setPreview(res.rendered);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      setPreviewErr(msg || 'Preview failed');
    }
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-3">
      <div>
        <label htmlFor="template-name" className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('common.name')}</label>
        <input id="template-name" data-testid="template-name" {...register('name', { required: true })} className="mt-1 block w-full rounded-md" />
      </div>
      <div>
        <label htmlFor="template-description" className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('common.description')}</label>
        <input id="template-description" data-testid="template-description" {...register('description')} className="mt-1 block w-full rounded-md" />
      </div>
      <div>
        <label htmlFor="template-type" className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.templateType')}</label>
        <select id="template-type" data-testid="template-type" {...register('template')} className="mt-1 block w-full rounded-md">
          <option value="minimal">{t('notificationProviders.minimal')}</option>
          <option value="detailed">{t('notificationProviders.detailed')}</option>
          <option value="custom">{t('notificationProviders.custom')}</option>
        </select>
      </div>
      <div>
        <label htmlFor="template-config" className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.configJson')}</label>
        <textarea id="template-config" data-testid="template-config" {...register('config')} rows={6} className="mt-1 block w-full font-mono text-xs rounded-md" />
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose} data-testid="template-cancel-btn">{t('common.cancel')}</Button>
        <Button type="button" variant="secondary" onClick={handlePreview} data-testid="template-preview-btn">{t('notificationProviders.preview')}</Button>
        <Button type="submit" data-testid="template-save-btn">{t('common.save')}</Button>
      </div>
      {previewErr && <div className="text-sm text-red-600">{previewErr}</div>}
      {preview && (
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">{t('notificationProviders.preview')}</label>
          <pre className="mt-1 p-2 bg-gray-50 dark:bg-gray-800 rounded text-xs overflow-auto whitespace-pre-wrap">{preview}</pre>
        </div>
      )}
    </form>
  );
};

const Notifications: FC = () => {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [isAdding, setIsAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [managingTemplates, setManagingTemplates] = useState(false);
  const [editingTemplateId, setEditingTemplateId] = useState<string | null>(null);

  const { data: providers, isLoading } = useQuery({
    queryKey: ['notificationProviders'],
    queryFn: getProviders,
  });

  const { data: externalTemplates } = useQuery({ queryKey: ['externalTemplates'], queryFn: getExternalTemplates });

  const createMutation = useMutation({
    mutationFn: createProvider,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notificationProviders'] });
      setIsAdding(false);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<NotificationProvider> }) => updateProvider(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notificationProviders'] });
      setEditingId(null);
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteProvider,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notificationProviders'] });
    },
  });

  const createTemplateMutation = useMutation({
    mutationFn: (data: Partial<ExternalTemplate>) => createExternalTemplate(data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['externalTemplates'] }),
  });

  const updateTemplateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Partial<ExternalTemplate> }) => updateExternalTemplate(id, data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['externalTemplates'] }),
  });

  const deleteTemplateMutation = useMutation({
    mutationFn: (id: string) => deleteExternalTemplate(id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['externalTemplates'] }),
  });

  const testMutation = useMutation({
    mutationFn: testProvider,
    onSuccess: () => alert(t('notificationProviders.testSent')),
    onError: (err: Error) => alert(`${t('notificationProviders.testFailed')}: ${err.message}`),
  });

  if (isLoading) return <div>{t('common.loading')}</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <Bell className="w-6 h-6" />
          {t('notificationProviders.title')}
        </h1>
        <Button onClick={() => setIsAdding(true)} data-testid="add-provider-btn">
          <Plus className="w-4 h-4 mr-2" />
          {t('notificationProviders.addProvider')}
        </Button>
      </div>

      {/* External Templates Management */}
      <div className="flex justify-between items-center">
        <h2 className="text-xl font-semibold text-gray-900 dark:text-white">{t('notificationProviders.externalTemplates')}</h2>
        <div className="flex items-center gap-2">
          <Button onClick={() => setManagingTemplates(!managingTemplates)} variant="secondary" size="sm">
            {managingTemplates ? t('notificationProviders.hideTemplates') : t('notificationProviders.manageTemplates')}
          </Button>
          <Button onClick={() => { setEditingTemplateId(null); setManagingTemplates(true); }}>
            <Plus className="w-4 h-4 mr-2" />{t('notificationProviders.newTemplate')}
          </Button>
        </div>
      </div>

      {managingTemplates && (
        <div className="space-y-4">
          {/* Template Form area */}
          {editingTemplateId !== null && (
            <Card className="p-4">
              <TemplateForm
                initialData={externalTemplates?.find((t: ExternalTemplate) => t.id === editingTemplateId) as Partial<ExternalTemplate>}
                onClose={() => setEditingTemplateId(null)}
                onSubmit={(data) => {
                  if (editingTemplateId) updateTemplateMutation.mutate({ id: editingTemplateId, data });
                  else createTemplateMutation.mutate(data as Partial<ExternalTemplate>);
                }}
              />
            </Card>
          )}

          {/* Create new when editingTemplateId is null and Manage Templates open -> show form */}
          {editingTemplateId === null && (
            <Card className="p-4">
              <h3 className="font-medium mb-2">{t('notificationProviders.createTemplate')}</h3>
              <TemplateForm
                onClose={() => setManagingTemplates(false)}
                onSubmit={(data) => createTemplateMutation.mutate(data as Partial<ExternalTemplate>)}
              />
            </Card>
          )}

          {/* List of templates */}
          <div className="grid gap-3">
            {externalTemplates?.map((t_template: ExternalTemplate) => (
              <Card key={t_template.id} className="p-4 flex justify-between items-start">
                <div>
                  <h4 className="font-medium text-gray-900 dark:text-white">{t_template.name}</h4>
                  <p className="text-sm text-gray-500 mt-1">{t_template.description}</p>
                  <pre className="mt-2 text-xs font-mono bg-gray-50 dark:bg-gray-800 p-2 rounded max-h-44 overflow-auto">{t_template.config}</pre>
                </div>
                <div className="flex flex-col gap-2 ml-4">
                  <Button size="sm" variant="secondary" onClick={() => setEditingTemplateId(t_template.id)}>
                    <Edit2 className="w-4 h-4" />
                  </Button>
                  <Button size="sm" variant="danger" onClick={() => { if (confirm(t('notificationProviders.deleteTemplateConfirm'))) deleteTemplateMutation.mutate(t_template.id); }}>
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </Card>
            ))}
            {externalTemplates?.length === 0 && (
              <div className="text-sm text-gray-500">{t('notificationProviders.noExternalTemplates')}</div>
            )}
          </div>
        </div>
      )}

      {isAdding && (
        <Card className="p-6 mb-6 border-blue-500 border-2">
          <h3 className="text-lg font-medium mb-4">{t('notificationProviders.addNewProvider')}</h3>
          <ProviderForm
            onClose={() => setIsAdding(false)}
            onSubmit={(data) => createMutation.mutate(data)}
          />
        </Card>
      )}

      <div className="grid gap-4">
        {providers?.map((provider) => (
          <Card key={provider.id} className="p-4">
            {editingId === provider.id ? (
              <ProviderForm
                initialData={provider}
                onClose={() => setEditingId(null)}
                onSubmit={(data) => updateMutation.mutate({ id: provider.id, data })}
              />
            ) : (
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className={`p-2 rounded-full ${provider.enabled ? 'bg-green-100 text-green-600' : 'bg-gray-100 text-gray-400'}`}>
                    <Bell className="w-5 h-5" />
                  </div>
                  <div>
                    <h3 className="font-medium text-gray-900 dark:text-white">{provider.name}</h3>
                    <div className="text-sm text-gray-500 dark:text-gray-400 flex items-center gap-2">
                      <span className="uppercase text-xs font-bold bg-gray-100 dark:bg-gray-700 px-2 py-0.5 rounded">
                        {provider.type}
                      </span>
                      <span className="truncate max-w-xs opacity-50">{provider.url}</span>
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => testMutation.mutate(provider)}
                    isLoading={testMutation.isPending}
                    title={t('notificationProviders.sendTest')}
                  >
                    <Send className="w-4 h-4" />
                  </Button>
                  <Button variant="secondary" size="sm" onClick={() => setEditingId(provider.id)}>
                    <Edit2 className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="danger"
                    size="sm"
                    onClick={() => {
                      if (confirm(t('notificationProviders.deleteConfirm'))) deleteMutation.mutate(provider.id);
                    }}
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            )}
          </Card>
        ))}

        {providers?.length === 0 && !isAdding && (
          <div className="text-center py-12 text-gray-500 bg-gray-50 dark:bg-gray-800 rounded-lg border-2 border-dashed border-gray-200 dark:border-gray-700">
            {t('notificationProviders.noProviders')}
          </div>
        )}
      </div>
    </div>
  );
};

export default Notifications;
