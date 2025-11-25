import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getForwardAuthConfig, updateForwardAuthConfig, getForwardAuthTemplates, ForwardAuthConfig } from '../api/security';
import { Button } from './ui/Button';
import { toast } from 'react-hot-toast';
import { Shield, Check, AlertTriangle } from 'lucide-react';

export default function ForwardAuthSettings() {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<ForwardAuthConfig>({
    provider: 'custom',
    address: '',
    trust_forward_header: true,
  });

  const { data: config, isLoading } = useQuery({
    queryKey: ['forwardAuth'],
    queryFn: getForwardAuthConfig,
  });

  const { data: templates } = useQuery({
    queryKey: ['forwardAuthTemplates'],
    queryFn: getForwardAuthTemplates,
  });

  useEffect(() => {
    if (config) {
      setFormData(config);
    }
  }, [config]);

  const mutation = useMutation({
    mutationFn: updateForwardAuthConfig,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['forwardAuth'] });
      toast.success('Forward Auth configuration saved');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to save configuration');
    },
  });

  const handleTemplateChange = (provider: string) => {
    if (templates && templates[provider]) {
      const template = templates[provider];
      setFormData({
        ...formData,
        provider: provider as any,
        address: template.address,
        trust_forward_header: template.trust_forward_header,
      });
    } else {
      setFormData({
        ...formData,
        provider: 'custom',
      });
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    mutation.mutate(formData);
  };

  if (isLoading) {
    return <div className="animate-pulse h-64 bg-gray-100 dark:bg-gray-800 rounded-lg"></div>;
  }

  return (
    <div className="bg-white dark:bg-dark-card rounded-lg shadow-sm border border-gray-200 dark:border-gray-800 p-6">
      <div className="flex items-center gap-3 mb-6">
        <div className="p-2 bg-blue-100 dark:bg-blue-900/30 rounded-lg">
          <Shield className="w-6 h-6 text-blue-600 dark:text-blue-400" />
        </div>
        <div>
          <h2 className="text-lg font-semibold text-gray-900 dark:text-white">Forward Authentication</h2>
          <p className="text-sm text-gray-500 dark:text-gray-400">
            Configure a global authentication provider (SSO) for your proxy hosts.
          </p>
        </div>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Provider Template
            </label>
            <select
              value={formData.provider}
              onChange={(e) => handleTemplateChange(e.target.value)}
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-dark-bg text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            >
              <option value="custom">Custom</option>
              <option value="authelia">Authelia</option>
              <option value="authentik">Authentik</option>
              <option value="pomerium">Pomerium</option>
            </select>
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              Select a template to pre-fill configuration or choose Custom.
            </p>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
              Auth Service Address
            </label>
            <input
              type="url"
              required
              value={formData.address}
              onChange={(e) => setFormData({ ...formData, address: e.target.value })}
              placeholder="http://authelia:9091/api/verify"
              className="w-full px-3 py-2 rounded-lg border border-gray-300 dark:border-gray-700 bg-white dark:bg-dark-bg text-gray-900 dark:text-white focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            />
            <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
              The internal URL where Caddy will send auth subrequests.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-3 p-4 bg-gray-50 dark:bg-gray-800/50 rounded-lg border border-gray-200 dark:border-gray-700">
          <input
            type="checkbox"
            id="trust_forward_header"
            checked={formData.trust_forward_header}
            onChange={(e) => setFormData({ ...formData, trust_forward_header: e.target.checked })}
            className="w-4 h-4 text-blue-600 rounded border-gray-300 focus:ring-blue-500 dark:bg-dark-bg dark:border-gray-600"
          />
          <label htmlFor="trust_forward_header" className="flex-1">
            <span className="block text-sm font-medium text-gray-900 dark:text-white">
              Trust Forward Headers
            </span>
            <span className="block text-xs text-gray-500 dark:text-gray-400">
              Send X-Forwarded-Method and X-Forwarded-Uri headers to the auth service. Required for most providers.
            </span>
          </label>
        </div>

        <div className="flex items-center justify-between pt-4 border-t border-gray-200 dark:border-gray-800">
          <div className="flex items-center gap-2 text-sm text-amber-600 dark:text-amber-400">
            <AlertTriangle className="w-4 h-4" />
            <span>Changes apply immediately to all hosts using Forward Auth.</span>
          </div>
          <Button
            type="submit"
            isLoading={mutation.isPending}
            className="flex items-center gap-2"
          >
            <Check className="w-4 h-4" />
            Save Configuration
          </Button>
        </div>
      </form>
    </div>
  );
}
