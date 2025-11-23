import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getProviders, createProvider, updateProvider, deleteProvider, testProvider, NotificationProvider } from '../api/notifications';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Bell, Plus, Trash2, Edit2, Send } from 'lucide-react';
import { useForm } from 'react-hook-form';

const ProviderForm: React.FC<{ 
  initialData?: Partial<NotificationProvider>; 
  onClose: () => void; 
  onSubmit: (data: any) => void;
}> = ({ initialData, onClose, onSubmit }) => {
  const { register, handleSubmit, watch, setValue, formState: { errors } } = useForm({
    defaultValues: initialData || { 
      type: 'discord', 
      enabled: true, 
      config: '',
      notify_proxy_hosts: true,
      notify_remote_servers: true,
      notify_domains: true,
      notify_certs: true,
      notify_uptime: true
    }
  });

  const type = watch('type');

  const setTemplate = (template: string) => {
    setValue('config', template);
  };

  return (
    <form onSubmit={handleSubmit(onSubmit)} className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Name</label>
        <input
          {...register('name', { required: 'Name is required' })}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
        />
        {errors.name && <span className="text-red-500 text-xs">{errors.name.message as string}</span>}
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">Type</label>
        <select
          {...register('type')}
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
        >
          <option value="discord">Discord</option>
          <option value="slack">Slack</option>
          <option value="gotify">Gotify</option>
          <option value="telegram">Telegram</option>
          <option value="generic">Generic Webhook (Shoutrrr)</option>
          <option value="webhook">Custom Webhook (JSON)</option>
        </select>
      </div>

      <div>
        <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">URL / Webhook</label>
        <input
          {...register('url', { required: 'URL is required' })}
          placeholder="https://discord.com/api/webhooks/..."
          className="mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white sm:text-sm"
        />
        {type !== 'webhook' && (
          <p className="text-xs text-gray-500 mt-1">
            For Shoutrrr format, see <a href="https://containrrr.dev/shoutrrr/" target="_blank" rel="noreferrer" className="text-blue-500 hover:underline">documentation</a>.
          </p>
        )}
      </div>

      {type === 'webhook' && (
        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300">JSON Payload Template</label>
          <div className="flex gap-2 mb-2 mt-1">
            <Button type="button" size="sm" variant="secondary" onClick={() => setTemplate('{"content": "{{.Title}}: {{.Message}}"}')}>
              Simple Template
            </Button>
            <Button type="button" size="sm" variant="secondary" onClick={() => setTemplate(`{
  "embeds": [{
    "title": "{{.Title}}",
    "description": "{{.Message}}",
    "color": 15158332,
    "fields": [
      { "name": "Monitor", "value": "{{.Name}}", "inline": true },
      { "name": "Status", "value": "{{.Status}}", "inline": true },
      { "name": "Latency", "value": "{{.Latency}}ms", "inline": true }
    ]
  }]
}`)}>
              Detailed Template (Discord)
            </Button>
          </div>
          <textarea
            {...register('config')}
            rows={8}
            className="mt-1 block w-full font-mono text-xs rounded-md border-gray-300 shadow-sm focus:border-blue-500 focus:ring-blue-500 dark:bg-gray-700 dark:border-gray-600 dark:text-white"
            placeholder='{"text": "{{.Message}}"}'
          />
          <p className="text-xs text-gray-500 mt-1">
            Available variables: .Title, .Message, .Status, .Name, .Latency, .Time
          </p>
        </div>
      )}

      <div className="space-y-2 border-t border-gray-200 dark:border-gray-700 pt-4">
        <h4 className="text-sm font-medium text-gray-900 dark:text-white">Notification Events</h4>
        <div className="grid grid-cols-2 gap-2">
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_proxy_hosts')} className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">Proxy Hosts</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_remote_servers')} className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">Remote Servers</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_domains')} className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">Domains</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_certs')} className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">Certificates</label>
          </div>
          <div className="flex items-center">
            <input type="checkbox" {...register('notify_uptime')} className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded" />
            <label className="ml-2 block text-sm text-gray-700 dark:text-gray-300">Uptime</label>
          </div>
        </div>
      </div>

      <div className="flex items-center">
        <input
          type="checkbox"
          {...register('enabled')}
          className="h-4 w-4 text-blue-600 focus:ring-blue-500 border-gray-300 rounded"
        />
        <label className="ml-2 block text-sm text-gray-900 dark:text-gray-300">Enabled</label>
      </div>

      <div className="flex justify-end gap-2 pt-4">
        <Button variant="secondary" onClick={onClose}>Cancel</Button>
        <Button type="submit">Save</Button>
      </div>
    </form>
  );
};

const Notifications: React.FC = () => {
  const queryClient = useQueryClient();
  const [isAdding, setIsAdding] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);

  const { data: providers, isLoading } = useQuery({
    queryKey: ['notificationProviders'],
    queryFn: getProviders,
  });

  const createMutation = useMutation({
    mutationFn: createProvider,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['notificationProviders'] });
      setIsAdding(false);
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, data }: { id: string; data: any }) => updateProvider(id, data),
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

  const testMutation = useMutation({
    mutationFn: testProvider,
    onSuccess: () => alert('Test notification sent!'),
    onError: (err: any) => alert(`Failed to send test: ${err.response?.data?.error || err.message}`),
  });

  if (isLoading) return <div>Loading...</div>;

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white flex items-center gap-2">
          <Bell className="w-6 h-6" />
          Notification Providers
        </h1>
        <Button onClick={() => setIsAdding(true)}>
          <Plus className="w-4 h-4 mr-2" />
          Add Provider
        </Button>
      </div>

      {isAdding && (
        <Card className="p-6 mb-6 border-blue-500 border-2">
          <h3 className="text-lg font-medium mb-4">Add New Provider</h3>
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
                    title="Send Test Notification"
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
                      if (confirm('Are you sure?')) deleteMutation.mutate(provider.id);
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
            No notification providers configured.
          </div>
        )}
      </div>
    </div>
  );
};

export default Notifications;
