import { useState } from 'react';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { Switch } from './ui/Switch';
import { X, Plus, ExternalLink, Shield, AlertTriangle, Info, Download, Trash2 } from 'lucide-react';
import type { AccessList, AccessListRule } from '../api/accessLists';
import { SECURITY_PRESETS, calculateTotalIPs, formatIPCount, type SecurityPreset } from '../data/securityPresets';
import { getMyIP } from '../api/system';
import toast from 'react-hot-toast';

interface AccessListFormProps {
  initialData?: AccessList;
  onSubmit: (data: AccessListFormData) => void;
  onCancel: () => void;
  onDelete?: () => void;
  isLoading?: boolean;
  isDeleting?: boolean;
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

export function AccessListForm({ initialData, onSubmit, onCancel, onDelete, isLoading, isDeleting }: AccessListFormProps) {
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
  const [showPresets, setShowPresets] = useState(false);
  const [loadingMyIP, setLoadingMyIP] = useState(false);

  const isGeoType = formData.type.startsWith('geo_');
  const isIPType = !isGeoType;

  // Calculate total IPs in current rules
  const totalIPs = isIPType && !formData.local_network_only
    ? calculateTotalIPs(ipRules.map(r => r.cidr))
    : 0;

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

  const handleApplyPreset = (preset: SecurityPreset) => {
    if (preset.type === 'geo_blacklist' && preset.countryCodes) {
      setFormData({ ...formData, type: 'geo_blacklist' });
      setSelectedCountries([...new Set([...selectedCountries, ...preset.countryCodes])]);
      toast.success(`Applied preset: ${preset.name}`);
    } else if (preset.type === 'blacklist' && preset.ipRanges) {
      setFormData({ ...formData, type: 'blacklist' });
      const newRules = preset.ipRanges.filter(
        (newRule) => !ipRules.some((existing) => existing.cidr === newRule.cidr)
      );
      setIPRules([...ipRules, ...newRules]);
      toast.success(`Applied preset: ${preset.name} (${newRules.length} rules added)`);
    }
    setShowPresets(false);
  };

  const handleGetMyIP = async () => {
    setLoadingMyIP(true);
    try {
      const result = await getMyIP();
      setNewIP(result.ip);
      toast.success(`Your IP: ${result.ip} (from ${result.source})`);
    } catch {
      toast.error('Failed to fetch your IP address');
    } finally {
      setLoadingMyIP(false);
    }
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
              href="https://wikid82.github.io/charon/security#acl-best-practices-by-service-type"
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
            <option value="blacklist">� IP Blacklist (Block Only) - Recommended</option>
            <option value="geo_whitelist">🌍 Geo Whitelist (Allow Countries)</option>
            <option value="geo_blacklist">🌍 Geo Blacklist (Block Countries) - Recommended</option>
          </select>
          {(formData.type === 'blacklist' || formData.type === 'geo_blacklist') && (
            <div className="mt-2 flex items-start gap-2 p-3 bg-blue-900/20 border border-blue-700/50 rounded-lg">
              <Info className="h-4 w-4 text-blue-400 mt-0.5 flex-shrink-0" />
              <p className="text-xs text-blue-300">
                <strong>Recommended:</strong> Block lists are safer than allow lists. They block known bad actors while allowing everyone else access, preventing lockouts.
              </p>
            </div>
          )}
        </div>

        {/* Security Presets */}
        {(formData.type === 'blacklist' || formData.type === 'geo_blacklist') && (
          <div className="bg-gray-800/50 border border-gray-700 rounded-lg p-4">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-2">
                <Shield className="h-5 w-5 text-green-400" />
                <h3 className="text-sm font-medium text-gray-300">Security Presets</h3>
              </div>
              <Button
                type="button"
                variant="secondary"
                size="sm"
                onClick={() => setShowPresets(!showPresets)}
              >
                {showPresets ? 'Hide' : 'Show'} Presets
              </Button>
            </div>

            {showPresets && (
              <div className="space-y-3 mt-4">
                <p className="text-xs text-gray-400 mb-3">
                  Quick-start templates based on threat intelligence feeds and best practices. Hover over (i) for data sources.
                </p>

                {/* Security Category - filter by current type */}
                <div>
                  <h4 className="text-xs font-semibold text-gray-400 uppercase mb-2">Recommended Security Presets</h4>
                  <div className="space-y-2">
                    {SECURITY_PRESETS.filter(p => p.category === 'security' && p.type === formData.type).map((preset) => (
                      <div
                        key={preset.id}
                        className="bg-gray-900 border border-gray-700 rounded-lg p-3 hover:border-gray-600 transition-colors"
                      >
                        <div className="flex items-start justify-between">
                          <div className="flex-1">
                            <div className="flex items-center gap-2 mb-1">
                              <h5 className="text-sm font-medium text-white">{preset.name}</h5>
                              <a
                                href={preset.dataSourceUrl}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-gray-400 hover:text-blue-400"
                                title={`Data from: ${preset.dataSource}`}
                              >
                                <Info className="h-3 w-3" />
                              </a>
                            </div>
                            <p className="text-xs text-gray-400 mb-2">{preset.description}</p>
                            <div className="flex items-center gap-3 text-xs">
                              <span className="text-gray-500">~{preset.estimatedIPs} IPs</span>
                              <span className="text-gray-600">|</span>
                              <span className="text-gray-500">{preset.dataSource}</span>
                            </div>
                            {preset.warning && (
                              <div className="flex items-start gap-1 mt-2 text-xs text-orange-400">
                                <AlertTriangle className="h-3 w-3 mt-0.5 flex-shrink-0" />
                                <span>{preset.warning}</span>
                              </div>
                            )}
                          </div>
                          <Button
                            type="button"
                            size="sm"
                            onClick={() => handleApplyPreset(preset)}
                            className="ml-3"
                          >
                            Apply
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Advanced Category - filter by current type */}
                <div>
                  <h4 className="text-xs font-semibold text-gray-400 uppercase mb-2">Advanced Presets</h4>
                  <div className="space-y-2">
                    {SECURITY_PRESETS.filter(p => p.category === 'advanced' && p.type === formData.type).map((preset) => (
                      <div
                        key={preset.id}
                        className="bg-gray-900 border border-gray-700 rounded-lg p-3 hover:border-gray-600 transition-colors"
                      >
                        <div className="flex items-start justify-between">
                          <div className="flex-1">
                            <div className="flex items-center gap-2 mb-1">
                              <h5 className="text-sm font-medium text-white">{preset.name}</h5>
                              <a
                                href={preset.dataSourceUrl}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-gray-400 hover:text-blue-400"
                                title={`Data from: ${preset.dataSource}`}
                              >
                                <Info className="h-3 w-3" />
                              </a>
                            </div>
                            <p className="text-xs text-gray-400 mb-2">{preset.description}</p>
                            <div className="flex items-center gap-3 text-xs">
                              <span className="text-gray-500">~{preset.estimatedIPs} IPs</span>
                              <span className="text-gray-600">|</span>
                              <span className="text-gray-500">{preset.dataSource}</span>
                            </div>
                            {preset.warning && (
                              <div className="flex items-start gap-1 mt-2 text-xs text-orange-400">
                                <AlertTriangle className="h-3 w-3 mt-0.5 flex-shrink-0" />
                                <span>{preset.warning}</span>
                              </div>
                            )}
                          </div>
                          <Button
                            type="button"
                            size="sm"
                            variant="secondary"
                            onClick={() => handleApplyPreset(preset)}
                            className="ml-3"
                          >
                            Apply
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            )}
          </div>
        )}

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
                <div className="mb-2 text-xs text-gray-500">
                  Note: IP-based blocklists (botnets, cloud scanners, VPN ranges) are better handled by CrowdSec, WAF, or rate limiting. Use IP-based ACLs sparingly for static or known ranges.
                </div>
                <div className="space-y-2">
                  <div className="flex items-center justify-between mb-2">
                    <label className="block text-sm font-medium text-gray-300">IP Addresses / CIDR Ranges</label>
                    <Button
                      type="button"
                      variant="secondary"
                      size="sm"
                      onClick={handleGetMyIP}
                      disabled={loadingMyIP}
                      className="flex items-center gap-1"
                    >
                      <Download className="h-3 w-3" />
                      {loadingMyIP ? 'Loading...' : 'Get My IP'}
                    </Button>
                  </div>
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
                  {totalIPs > 0 && (
                    <div className="flex items-center gap-2 text-xs text-gray-400">
                      <Info className="h-3 w-3" />
                      <span>Current rules cover approximately <strong className="text-white">{formatIPCount(totalIPs)}</strong> IP addresses</span>
                    </div>
                  )}
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
      <div className="flex justify-between gap-2">
        <div>
          {initialData && onDelete && (
            <Button
              type="button"
              variant="danger"
              onClick={onDelete}
              disabled={isLoading || isDeleting}
            >
              <Trash2 className="h-4 w-4 mr-2" />
              {isDeleting ? 'Deleting...' : 'Delete'}
            </Button>
          )}
        </div>
        <div className="flex gap-2">
          <Button type="button" variant="secondary" onClick={onCancel} disabled={isLoading || isDeleting}>
            Cancel
          </Button>
          <Button type="submit" variant="primary" disabled={isLoading || isDeleting}>
            {isLoading ? 'Saving...' : initialData ? 'Update' : 'Create'}
          </Button>
        </div>
      </div>
    </form>
  );
}
