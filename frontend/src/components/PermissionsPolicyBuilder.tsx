import { useState, useEffect } from 'react';
import { Plus, X, Code } from 'lucide-react';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { NativeSelect } from './ui/NativeSelect';
import { Card } from './ui/Card';
import { Badge } from './ui/Badge';
import { Alert } from './ui/Alert';

interface PermissionsPolicyItem {
  feature: string;
  allowlist: string[];
}

interface PermissionsPolicyBuilderProps {
  value: string; // JSON string of PermissionsPolicyItem[]
  onChange: (value: string) => void;
}

const FEATURES = [
  'accelerometer',
  'ambient-light-sensor',
  'autoplay',
  'battery',
  'camera',
  'display-capture',
  'document-domain',
  'encrypted-media',
  'fullscreen',
  'geolocation',
  'gyroscope',
  'magnetometer',
  'microphone',
  'midi',
  'payment',
  'picture-in-picture',
  'publickey-credentials-get',
  'screen-wake-lock',
  'sync-xhr',
  'usb',
  'web-share',
  'xr-spatial-tracking',
];

const ALLOWLIST_PRESETS = [
  { label: 'None (disable)', value: '' },
  { label: 'Self', value: 'self' },
  { label: 'All (*)', value: '*' },
];

export function PermissionsPolicyBuilder({ value, onChange }: PermissionsPolicyBuilderProps) {
  const [policies, setPolicies] = useState<PermissionsPolicyItem[]>([]);
  const [newFeature, setNewFeature] = useState('camera');
  const [newAllowlist, setNewAllowlist] = useState('');
  const [customOrigin, setCustomOrigin] = useState('');
  const [showPreview, setShowPreview] = useState(false);

  // Parse initial value
  useEffect(() => {
    try {
      if (value) {
        const parsed = JSON.parse(value) as PermissionsPolicyItem[];
        setPolicies(parsed);
      } else {
        setPolicies([]);
      }
    } catch {
      setPolicies([]);
    }
  }, [value]);

  // Generate Permissions-Policy string preview
  const generatePolicyString = (pols: PermissionsPolicyItem[]): string => {
    return pols
      .map((pol) => {
        if (pol.allowlist.length === 0) {
          return `${pol.feature}=()`;
        }
        const allowlistStr = pol.allowlist.join(' ');
        return `${pol.feature}=(${allowlistStr})`;
      })
      .join(', ');
  };

  const policyString = generatePolicyString(policies);

  // Update parent component
  const updatePolicies = (newPolicies: PermissionsPolicyItem[]) => {
    setPolicies(newPolicies);
    onChange(JSON.stringify(newPolicies));
  };

  const handleAddFeature = () => {
    const existingIndex = policies.findIndex((p) => p.feature === newFeature);

    let allowlist: string[] = [];
    if (newAllowlist === 'self') {
      allowlist = ['self'];
    } else if (newAllowlist === '*') {
      allowlist = ['*'];
    } else if (customOrigin.trim()) {
      allowlist = [customOrigin.trim()];
    }

    if (existingIndex >= 0) {
      // Update existing
      const updated = [...policies];
      updated[existingIndex] = { feature: newFeature, allowlist };
      updatePolicies(updated);
    } else {
      // Add new
      updatePolicies([...policies, { feature: newFeature, allowlist }]);
    }

    setCustomOrigin('');
  };

  const handleRemoveFeature = (feature: string) => {
    updatePolicies(policies.filter((p) => p.feature !== feature));
  };

  const handleQuickAdd = (features: string[]) => {
    const newPolicies = features.map((feature) => ({
      feature,
      allowlist: [],
    }));

    // Merge with existing (don't duplicate)
    const merged = [...policies];
    newPolicies.forEach((newPolicy) => {
      if (!merged.some((p) => p.feature === newPolicy.feature)) {
        merged.push(newPolicy);
      }
    });

    updatePolicies(merged);
  };

  return (
    <Card className="p-4 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Permissions Policy Builder</h3>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setShowPreview(!showPreview)}
        >
          <Code className="w-4 h-4 mr-2" />
          {showPreview ? 'Hide' : 'Show'} Preview
        </Button>
      </div>

      {/* Quick Add Buttons */}
      <div className="space-y-2">
        <span className="text-sm text-gray-600 dark:text-gray-400">Quick Add:</span>
        <div className="flex flex-wrap gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleQuickAdd(['camera', 'microphone', 'geolocation'])}
          >
            Disable Common Features
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => handleQuickAdd(['payment', 'usb', 'midi'])}
          >
            Disable Sensitive APIs
          </Button>
        </div>
      </div>

      {/* Add Feature Form */}
      <div className="space-y-2">
        <div className="flex gap-2">
          <NativeSelect
            value={newFeature}
            onChange={(e) => setNewFeature(e.target.value)}
            className="w-48"
            aria-label="Select Feature"
          >
            {FEATURES.map((feature) => (
              <option key={feature} value={feature}>
                {feature}
              </option>
            ))}
          </NativeSelect>

          <NativeSelect
            value={newAllowlist}
            onChange={(e) => setNewAllowlist(e.target.value)}
            className="w-40"
            aria-label="Select Allowlist Origin"
          >
            {ALLOWLIST_PRESETS.map((preset) => (
              <option key={preset.value} value={preset.value}>
                {preset.label}
              </option>
            ))}
          </NativeSelect>

          {newAllowlist === '' && (
            <Input
              type="text"
              value={customOrigin}
              onChange={(e) => setCustomOrigin(e.target.value)}
              placeholder="or enter origin (e.g., https://example.com)"
              className="flex-1"
            />
          )}

          <Button onClick={handleAddFeature} aria-label="Add Feature">
            <Plus className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {/* Current Policies */}
      <div className="space-y-2">
        {policies.length === 0 ? (
          <Alert variant="info">
            <span>No permissions policies configured. Add features above to restrict browser capabilities.</span>
          </Alert>
        ) : (
          policies.map((policy) => (
            <div key={policy.feature} className="flex items-center gap-3 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
              <span className="font-mono text-sm font-semibold text-gray-900 dark:text-white flex-shrink-0">
                {policy.feature}
              </span>
              <div className="flex-1">
                {policy.allowlist.length === 0 ? (
                  <Badge variant="error">Disabled</Badge>
                ) : policy.allowlist.includes('*') ? (
                  <Badge variant="success">Allowed (all origins)</Badge>
                ) : policy.allowlist.includes('self') ? (
                  <Badge variant="outline">Self only</Badge>
                ) : (
                  <div className="flex flex-wrap gap-1">
                    {policy.allowlist.map((origin) => (
                      <Badge key={origin} variant="outline" className="font-mono text-xs">
                        {origin}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={() => handleRemoveFeature(policy.feature)}
                aria-label={`Remove ${policy.feature}`}
              >
                <X className="w-4 h-4" />
              </Button>
            </div>
          ))
        )}
      </div>

      {/* Policy String Preview */}
      {showPreview && policyString && (
        <div className="space-y-2">
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300">Generated Permissions-Policy Header:</label>
          <pre className="p-3 bg-gray-900 dark:bg-gray-950 text-green-400 rounded-lg overflow-x-auto text-xs font-mono">
            {policyString || '(empty)'}
          </pre>
        </div>
      )}
    </Card>
  );
}
