import { useState } from 'react';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Switch } from './ui/Switch';
import { X, Plus, ExternalLink } from 'lucide-react';
import type { AccessList, AccessListRule } from '../api/accessLists';

interface AccessListFormProps {
  initialData?: AccessList;
  onSubmit: (data: AccessListFormData) => void;
  onCancel: () => void;
  isLoading?: boolean;
}

export interface AccessListFormData {
  name: string;
  description: string;
  type: 'whitelist' | 'blacklist' | 'geo_whitelist' | 'geo_blacklist';
  ip_rules: string;
  country_codes: string;
  local_network_only: boolean;
  enabled: boolean;
}

const COUNTRIES = [
  { code: 'US', name: 'United States' },
  { code: 'CA', name: 'Canada' },
  { code: 'GB', name: 'United Kingdom' },
  { code: 'DE', name: 'Germany' },
  { code: 'FR', name: 'France' },
  { code: 'IT', name: 'Italy' },
  { code: 'ES', name: 'Spain' },
  { code: 'NL', name: 'Netherlands' },
  { code: 'BE', name: 'Belgium' },
  { code: 'SE', name: 'Sweden' },
  { code: 'NO', name: 'Norway' },
  { code: 'DK', name: 'Denmark' },
  { code: 'FI', name: 'Finland' },
  { code: 'PL', name: 'Poland' },
  { code: 'CZ', name: 'Czech Republic' },
  { code: 'AT', name: 'Austria' },
  { code: 'CH', name: 'Switzerland' },
  { code: 'AU', name: 'Australia' },
  { code: 'NZ', name: 'New Zealand' },
  { code: 'JP', name: 'Japan' },
  { code: 'CN', name: 'China' },
  { code: 'IN', name: 'India' },
  { code: 'BR', name: 'Brazil' },
  { code: 'MX', name: 'Mexico' },
  { code: 'AR', name: 'Argentina' },
  { code: 'RU', name: 'Russia' },
  { code: 'UA', name: 'Ukraine' },
  { code: 'TR', name: 'Turkey' },
  { code: 'IL', name: 'Israel' },
  { code: 'SA', name: 'Saudi Arabia' },
  { code: 'AE', name: 'United Arab Emirates' },
  { code: 'EG', name: 'Egypt' },
  { code: 'ZA', name: 'South Africa' },
  { code: 'KR', name: 'South Korea' },
  { code: 'SG', name: 'Singapore' },
  { code: 'MY', name: 'Malaysia' },
  { code: 'TH', name: 'Thailand' },
  { code: 'ID', name: 'Indonesia' },
  { code: 'PH', name: 'Philippines' },
  { code: 'VN', name: 'Vietnam' },
];

export function AccessListForm({ initialData, onSubmit, onCancel, isLoading }: AccessListFormProps) {
  const [formData, setFormData] = useState<AccessListFormData>({
    name: initialData?.name || '',
    description: initialData?.description || '',
    type: initialData?.type || 'whitelist',
    ip_rules: initialData?.ip_rules || '',
    country_codes: initialData?.country_codes || '',
    local_network_only: initialData?.local_network_only || false,
    enabled: initialData?.enabled ?? true,
  });

  const [ipRules, setIPRules] = useState<AccessListRule[]>(() => {
    if (initialData?.ip_rules) {
      try {
        return JSON.parse(initialData.ip_rules);
      } catch {
        return [];
      }
    }
    return [];
  });

  const [selectedCountries, setSelectedCountries] = useState<string[]>(() => {
    if (initialData?.country_codes) {
      return initialData.country_codes.split(',').map((c) => c.trim());
    }
    return [];
  });

  const [newIP, setNewIP] = useState('');
  const [newIPDescription, setNewIPDescription] = useState('');

  const isGeoType = formData.type.startsWith('geo_');
  const isIPType = !isGeoType;

  const handleAddIP = () => {
    if (!newIP.trim()) return;

    const newRule: AccessListRule = {
      cidr: newIP.trim(),
      description: newIPDescription.trim(),
    };

    const updatedRules = [...ipRules, newRule];
    setIPRules(updatedRules);
    setNewIP('');
    setNewIPDescription('');
  };

  const handleRemoveIP = (index: number) => {
    setIPRules(ipRules.filter((_, i) => i !== index));
  };

  const handleAddCountry = (countryCode: string) => {
    if (!selectedCountries.includes(countryCode)) {
      setSelectedCountries([...selectedCountries, countryCode]);
    }
  };

  const handleRemoveCountry = (countryCode: string) => {
    setSelectedCountries(selectedCountries.filter((c) => c !== countryCode));
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    const data: AccessListFormData = {
      ...formData,
      ip_rules: isIPType && !formData.local_network_only ? JSON.stringify(ipRules) : '',
      country_codes: isGeoType ? selectedCountries.join(',') : '',
    };

    onSubmit(data);
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {/* Basic Info */}
      <div className="space-y-4">
        <div>
          <label htmlFor="name" className="block text-sm font-medium text-gray-300 mb-2">
            Name *
          </label>
          <Input
            id="name"
            value={formData.name}
            onChange={(e) => setFormData({ ...formData, name: e.target.value })}
            placeholder="My Access List"
            required
          />
        </div>

        <div>
          <label htmlFor="description" className="block text-sm font-medium text-gray-300 mb-2">
            Description
          </label>
          <textarea
            id="description"
            value={formData.description}
            onChange={(e) => setFormData({ ...formData, description: e.target.value })}
            placeholder="Optional description"
            rows={2}
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>

        <div>
          <label htmlFor="type" className="block text-sm font-medium text-gray-300 mb-2">
            Type *
            <a
              href="https://wikid82.github.io/cpmp/docs/security.html#acl-best-practices-by-service-type"
              target="_blank"
              rel="noopener noreferrer"
              className="ml-2 text-blue-400 hover:text-blue-300 text-xs"
            >
              <ExternalLink className="inline h-3 w-3" /> Best Practices
            </a>
          </label>
          <select
            id="type"
            value={formData.type}
            onChange={(e) =>
              setFormData({ ...formData, type: e.target.value as 'whitelist' | 'blacklist' | 'geo_whitelist' | 'geo_blacklist', local_network_only: false })
            }
            className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="whitelist">🛡️ IP Whitelist (Allow Only)</option>
            <option value="blacklist">🛡️ IP Blacklist (Block Only)</option>
            <option value="geo_whitelist">🌍 Geo Whitelist (Allow Countries)</option>
            <option value="geo_blacklist">🌍 Geo Blacklist (Block Countries)</option>
          </select>
        </div>

        <div className="flex items-center justify-between">
          <div>
            <label className="block text-sm font-medium text-gray-300">Enabled</label>
            <p className="text-xs text-gray-500">Apply this access list to hosts</p>
          </div>
          <Switch
            checked={formData.enabled}
            onCheckedChange={(checked) => setFormData({ ...formData, enabled: checked })}
          />
        </div>
      </div>

      {/* IP-based Rules */}
      {isIPType && (
        <div className="bg-gray-800 border border-gray-700 rounded-lg p-6 space-y-4">
          <div className="flex items-center justify-between">
            <div>
              <label className="block text-sm font-medium text-gray-300">Local Network Only (RFC1918)</label>
              <p className="text-xs text-gray-500">
                Allow only private network IPs (10.x.x.x, 192.168.x.x, 172.16-31.x.x)
              </p>
            </div>
            <Switch
              checked={formData.local_network_only}
              onCheckedChange={(checked) =>
                setFormData({ ...formData, local_network_only: checked })
              }
            />
          </div>

            {!formData.local_network_only && (
              <>
                <div className="space-y-2">
                  <label className="block text-sm font-medium text-gray-300">IP Addresses / CIDR Ranges</label>
                  <div className="flex gap-2">
                    <Input
                      value={newIP}
                      onChange={(e) => setNewIP(e.target.value)}
                      placeholder="192.168.1.0/24 or 10.0.0.1"
                      onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), handleAddIP())}
                    />
                    <Input
                      value={newIPDescription}
                      onChange={(e) => setNewIPDescription(e.target.value)}
                      placeholder="Description (optional)"
                      className="flex-1"
                      onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), handleAddIP())}
                    />
                    <Button type="button" onClick={handleAddIP} size="sm">
                      <Plus className="h-4 w-4" />
                    </Button>
                  </div>
                </div>

                {ipRules.length > 0 && (
                  <div className="space-y-2">
                    {ipRules.map((rule, index) => (
                      <div
                        key={index}
                        className="flex items-center justify-between p-3 rounded-lg border border-gray-600 bg-gray-700"
                      >
                        <div>
                          <p className="font-mono text-sm text-white">{rule.cidr}</p>
                          {rule.description && (
                            <p className="text-xs text-gray-400">{rule.description}</p>
                          )}
                        </div>
                        <button
                          type="button"
                          onClick={() => handleRemoveIP(index)}
                          className="text-gray-400 hover:text-red-400"
                        >
                          <X className="h-4 w-4" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </>
            )}
        </div>
      )}

      {/* Geo-blocking Rules */}
      {isGeoType && (
        <div className="bg-gray-800 border border-gray-700 rounded-lg p-6 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-300 mb-2">Select Countries</label>
            <select
              onChange={(e) => {
                if (e.target.value) {
                  handleAddCountry(e.target.value);
                  e.target.value = '';
                }
              }}
              className="w-full bg-gray-900 border border-gray-700 rounded-lg px-4 py-2 text-white focus:outline-none focus:ring-2 focus:ring-blue-500"
            >
              <option value="">Add a country...</option>
              {COUNTRIES.filter((c) => !selectedCountries.includes(c.code)).map((country) => (
                <option key={country.code} value={country.code}>
                  {country.name} ({country.code})
                </option>
              ))}
            </select>
          </div>

          {selectedCountries.length > 0 && (
            <div className="flex flex-wrap gap-2">
              {selectedCountries.map((code) => {
                const country = COUNTRIES.find((c) => c.code === code);
                return (
                  <span
                    key={code}
                    className="inline-flex items-center gap-1 px-3 py-1 rounded-full text-sm bg-gray-700 text-gray-200 border border-gray-600"
                  >
                    {country?.name || code}
                    <X
                      className="h-3 w-3 cursor-pointer hover:text-red-400"
                      onClick={() => handleRemoveCountry(code)}
                    />
                  </span>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* Actions */}
      <div className="flex justify-end gap-2">
        <Button type="button" variant="secondary" onClick={onCancel} disabled={isLoading}>
          Cancel
        </Button>
        <Button type="submit" variant="primary" disabled={isLoading}>
          {isLoading ? 'Saving...' : initialData ? 'Update' : 'Create'}
        </Button>
      </div>
    </form>
  );
}
