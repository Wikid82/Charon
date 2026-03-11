import { AlertTriangle, Save, X } from 'lucide-react';
import { useState, useEffect } from 'react';

import { CSPBuilder } from './CSPBuilder';
import { PermissionsPolicyBuilder } from './PermissionsPolicyBuilder';
import { SecurityScoreDisplay } from './SecurityScoreDisplay';
import { Alert } from './ui/Alert';
import { Button } from './ui/Button';
import { Card } from './ui/Card';
import { Input } from './ui/Input';
import { NativeSelect } from './ui/NativeSelect';
import { Switch } from './ui/Switch';
import { Textarea } from './ui/Textarea';
import { useCalculateSecurityScore } from '../hooks/useSecurityHeaders';

import type { SecurityHeaderProfile, CreateProfileRequest } from '../api/securityHeaders';

interface SecurityHeaderProfileFormProps {
  initialData?: SecurityHeaderProfile;
  onSubmit: (data: CreateProfileRequest) => void;
  onCancel: () => void;
  onDelete?: () => void;
  isLoading?: boolean;
  isDeleting?: boolean;
}

export function SecurityHeaderProfileForm({
  initialData,
  onSubmit,
  onCancel,
  onDelete,
  isLoading,
  isDeleting,
}: SecurityHeaderProfileFormProps) {
  const [formData, setFormData] = useState<CreateProfileRequest>({
    name: initialData?.name || '',
    description: initialData?.description || '',
    hsts_enabled: initialData?.hsts_enabled ?? true,
    hsts_max_age: initialData?.hsts_max_age || 31536000,
    hsts_include_subdomains: initialData?.hsts_include_subdomains ?? true,
    hsts_preload: initialData?.hsts_preload ?? false,
    csp_enabled: initialData?.csp_enabled ?? false,
    csp_directives: initialData?.csp_directives || '',
    csp_report_only: initialData?.csp_report_only ?? false,
    csp_report_uri: initialData?.csp_report_uri || '',
    x_frame_options: initialData?.x_frame_options || 'DENY',
    x_content_type_options: initialData?.x_content_type_options ?? true,
    referrer_policy: initialData?.referrer_policy || 'strict-origin-when-cross-origin',
    permissions_policy: initialData?.permissions_policy || '',
    cross_origin_opener_policy: initialData?.cross_origin_opener_policy || 'same-origin',
    cross_origin_resource_policy: initialData?.cross_origin_resource_policy || 'same-origin',
    cross_origin_embedder_policy: initialData?.cross_origin_embedder_policy || '',
    xss_protection: initialData?.xss_protection ?? true,
    cache_control_no_store: initialData?.cache_control_no_store ?? false,
  });

  const [cspValid, setCspValid] = useState(true);
  const [, setCspErrors] = useState<string[]>([]);

  const calculateScoreMutation = useCalculateSecurityScore();
  const { mutate: calculateScore } = calculateScoreMutation;

  // Calculate score when form data changes
  useEffect(() => {
    const timer = setTimeout(() => {
      calculateScore(formData);
    }, 500);

    return () => clearTimeout(timer);
  }, [formData, calculateScore]);

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();

    if (!formData.name.trim()) {
      return;
    }

    onSubmit(formData);
  };

  const updateField = <K extends keyof CreateProfileRequest>(
    field: K,
    value: CreateProfileRequest[K]
  ) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
  };

  const isPreset = initialData?.is_preset ?? false;

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {/* Basic Info */}
      <Card className="p-4 space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Profile Information</h3>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Profile Name *
          </label>
          <Input
            type="text"
            value={formData.name}
            onChange={(e) => updateField('name', e.target.value)}
            placeholder="e.g., Production Security Headers"
            required
            disabled={isPreset}
          />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Description
          </label>
          <Textarea
            value={formData.description || ''}
            onChange={(e) => updateField('description', e.target.value)}
            placeholder="Optional description of this security profile..."
            rows={2}
            disabled={isPreset}
          />
        </div>

        {isPreset && (
          <Alert variant="info">
            This is a system preset and cannot be modified. Clone it to create a custom profile.
          </Alert>
        )}
      </Card>

      {/* Live Security Score */}
      {calculateScoreMutation.data && (
        <SecurityScoreDisplay
          score={calculateScoreMutation.data.score}
          maxScore={calculateScoreMutation.data.max_score}
          breakdown={calculateScoreMutation.data.breakdown}
          suggestions={calculateScoreMutation.data.suggestions}
          size="md"
          showDetails={true}
        />
      )}

      {/* HSTS Section */}
      <Card className="p-4 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
            HTTP Strict Transport Security (HSTS)
          </h3>
          <Switch
            checked={formData.hsts_enabled}
            onCheckedChange={(checked) => updateField('hsts_enabled', checked)}
            disabled={isPreset}
          />
        </div>

        {formData.hsts_enabled && (
          <>
            <div>
              <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                Max Age (seconds)
              </label>
              <Input
                type="number"
                value={formData.hsts_max_age}
                onChange={(e) => updateField('hsts_max_age', parseInt(e.target.value) || 0)}
                min={0}
                disabled={isPreset}
              />
              <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
                Recommended: 31536000 (1 year) or 63072000 (2 years)
              </p>
            </div>

            <div className="flex items-center justify-between">
              <div>
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Include Subdomains
                </label>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Apply HSTS to all subdomains
                </p>
              </div>
              <Switch
                checked={formData.hsts_include_subdomains}
                onCheckedChange={(checked) => updateField('hsts_include_subdomains', checked)}
                disabled={isPreset}
              />
            </div>

            <div className="flex items-center justify-between">
              <div>
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Preload
                </label>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Submit to browser preload lists
                </p>
              </div>
              <Switch
                checked={formData.hsts_preload}
                onCheckedChange={(checked) => updateField('hsts_preload', checked)}
                disabled={isPreset}
              />
            </div>

            {formData.hsts_preload && (
              <Alert variant="warning">
                <AlertTriangle className="w-4 h-4" />
                <div>
                  <p className="font-semibold">Warning: HSTS Preload is Permanent</p>
                  <p className="text-sm mt-1">
                    Once submitted to browser preload lists, removal can take months. Only enable if you're
                    committed to HTTPS forever.
                  </p>
                </div>
              </Alert>
            )}
          </>
        )}
      </Card>

      {/* CSP Section */}
      <Card className="p-4 space-y-4">
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold text-gray-900 dark:text-white">
            Content Security Policy (CSP)
          </h3>
          <Switch
            checked={formData.csp_enabled}
            onCheckedChange={(checked) => updateField('csp_enabled', checked)}
            disabled={isPreset}
          />
        </div>

        {formData.csp_enabled && (
          <>
            <CSPBuilder
              value={formData.csp_directives || ''}
              onChange={(value) => updateField('csp_directives', value)}
              onValidate={(valid, errors) => {
                setCspValid(valid);
                setCspErrors(errors);
              }}
            />

            <div className="flex items-center justify-between">
              <div>
                <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Report-Only Mode
                </label>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  Test CSP without blocking content
                </p>
              </div>
              <Switch
                checked={formData.csp_report_only}
                onCheckedChange={(checked) => updateField('csp_report_only', checked)}
                disabled={isPreset}
              />
            </div>

            {formData.csp_report_only && (
              <div>
                <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
                  Report URI (optional)
                </label>
                <Input
                  type="url"
                  value={formData.csp_report_uri || ''}
                  onChange={(e) => updateField('csp_report_uri', e.target.value)}
                  placeholder="https://example.com/csp-report"
                  disabled={isPreset}
                />
              </div>
            )}
          </>
        )}
      </Card>

      {/* Frame Options */}
      <Card className="p-4 space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Clickjacking Protection</h3>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            X-Frame-Options
          </label>
          <NativeSelect
            value={formData.x_frame_options}
            onChange={(e) => updateField('x_frame_options', e.target.value)}
            disabled={isPreset}
          >
            <option value="DENY">DENY (Recommended - no framing allowed)</option>
            <option value="SAMEORIGIN">SAMEORIGIN (allow same origin framing)</option>
            <option value="">None (allow all framing - not recommended)</option>
          </NativeSelect>
        </div>

        <div className="flex items-center justify-between">
          <div>
            <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
              X-Content-Type-Options: nosniff
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Prevent MIME type sniffing attacks
            </p>
          </div>
          <Switch
            checked={formData.x_content_type_options}
            onCheckedChange={(checked) => updateField('x_content_type_options', checked)}
            disabled={isPreset}
          />
        </div>
      </Card>

      {/* Privacy Headers */}
      <Card className="p-4 space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Privacy Controls</h3>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Referrer-Policy
          </label>
          <NativeSelect
            value={formData.referrer_policy}
            onChange={(e) => updateField('referrer_policy', e.target.value)}
            disabled={isPreset}
          >
            <option value="no-referrer">no-referrer (Most Private)</option>
            <option value="no-referrer-when-downgrade">no-referrer-when-downgrade</option>
            <option value="origin">origin</option>
            <option value="origin-when-cross-origin">origin-when-cross-origin</option>
            <option value="same-origin">same-origin</option>
            <option value="strict-origin">strict-origin</option>
            <option value="strict-origin-when-cross-origin">strict-origin-when-cross-origin (Recommended)</option>
            <option value="unsafe-url">unsafe-url (Least Private)</option>
          </NativeSelect>
        </div>
      </Card>

      {/* Permissions Policy */}
      <PermissionsPolicyBuilder
        value={formData.permissions_policy || ''}
        onChange={(value) => updateField('permissions_policy', value)}
      />

      {/* Cross-Origin Headers */}
      <Card className="p-4 space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Cross-Origin Isolation</h3>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Cross-Origin-Opener-Policy
          </label>
          <NativeSelect
            value={formData.cross_origin_opener_policy}
            onChange={(e) => updateField('cross_origin_opener_policy', e.target.value)}
            disabled={isPreset}
          >
            <option value="">None</option>
            <option value="unsafe-none">unsafe-none</option>
            <option value="same-origin-allow-popups">same-origin-allow-popups</option>
            <option value="same-origin">same-origin (Recommended)</option>
          </NativeSelect>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Cross-Origin-Resource-Policy
          </label>
          <NativeSelect
            value={formData.cross_origin_resource_policy}
            onChange={(e) => updateField('cross_origin_resource_policy', e.target.value)}
            disabled={isPreset}
          >
            <option value="">None</option>
            <option value="same-site">same-site</option>
            <option value="same-origin">same-origin (Recommended)</option>
            <option value="cross-origin">cross-origin</option>
          </NativeSelect>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-1">
            Cross-Origin-Embedder-Policy
          </label>
          <NativeSelect
            value={formData.cross_origin_embedder_policy}
            onChange={(e) => updateField('cross_origin_embedder_policy', e.target.value)}
            disabled={isPreset}
          >
            <option value="">None (Default)</option>
            <option value="require-corp">require-corp (Strict)</option>
          </NativeSelect>
          <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">
            Only enable if you need SharedArrayBuffer or high-resolution timers
          </p>
        </div>
      </Card>

      {/* Additional Options */}
      <Card className="p-4 space-y-4">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Additional Options</h3>

        <div className="flex items-center justify-between">
          <div>
            <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
              X-XSS-Protection
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Legacy XSS protection header
            </p>
          </div>
          <Switch
            checked={formData.xss_protection}
            onCheckedChange={(checked) => updateField('xss_protection', checked)}
            disabled={isPreset}
          />
        </div>

        <div className="flex items-center justify-between">
          <div>
            <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
              Cache-Control: no-store
            </label>
            <p className="text-xs text-gray-500 dark:text-gray-400">
              Prevent caching of sensitive content
            </p>
          </div>
          <Switch
            checked={formData.cache_control_no_store}
            onCheckedChange={(checked) => updateField('cache_control_no_store', checked)}
            disabled={isPreset}
          />
        </div>
      </Card>

      {/* Form Actions */}
      <div className="flex items-center justify-between pt-4 border-t border-gray-200 dark:border-gray-700">
        <div>
          {onDelete && !isPreset && (
            <Button
              type="button"
              variant="danger"
              onClick={onDelete}
              disabled={isDeleting}
            >
              {isDeleting ? 'Deleting...' : 'Delete Profile'}
            </Button>
          )}
        </div>

        <div className="flex gap-2">
          <Button type="button" variant="outline" onClick={onCancel} disabled={isLoading}>
            <X className="w-4 h-4 mr-2" />
            Cancel
          </Button>
          <Button
            type="submit"
            disabled={isLoading || isPreset || (!cspValid && formData.csp_enabled)}
          >
            <Save className="w-4 h-4 mr-2" />
            {isLoading ? 'Saving...' : 'Save Profile'}
          </Button>
        </div>
      </div>
    </form>
  );
}
