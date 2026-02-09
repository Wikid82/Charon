import { useState, useEffect } from 'react';
import { Plus, X, AlertCircle, Check, Code } from 'lucide-react';
import { Button } from './ui/Button';
import { Input } from './ui/Input';
import { NativeSelect } from './ui/NativeSelect';
import { Card } from './ui/Card';
import { Badge } from './ui/Badge';
import { Alert } from './ui/Alert';
import type { CSPDirective } from '../api/securityHeaders';

interface CSPBuilderProps {
  value: string; // JSON string of CSPDirective[]
  onChange: (value: string) => void;
  onValidate?: (valid: boolean, errors: string[]) => void;
}

const CSP_DIRECTIVES = [
  'default-src',
  'script-src',
  'style-src',
  'img-src',
  'font-src',
  'connect-src',
  'frame-src',
  'object-src',
  'media-src',
  'worker-src',
  'form-action',
  'base-uri',
  'frame-ancestors',
  'manifest-src',
  'prefetch-src',
];

const CSP_VALUES = [
  "'self'",
  "'none'",
  "'unsafe-inline'",
  "'unsafe-eval'",
  'data:',
  'https:',
  'http:',
  'blob:',
  'filesystem:',
  "'strict-dynamic'",
  "'report-sample'",
  "'unsafe-hashes'",
];

const CSP_PRESETS: Record<string, CSPDirective[]> = {
  'Strict Default': [
    { directive: 'default-src', values: ["'self'"] },
    { directive: 'script-src', values: ["'self'"] },
    { directive: 'style-src', values: ["'self'"] },
    { directive: 'img-src', values: ["'self'", 'data:', 'https:'] },
    { directive: 'font-src', values: ["'self'", 'data:'] },
    { directive: 'connect-src', values: ["'self'"] },
    { directive: 'frame-src', values: ["'none'"] },
    { directive: 'object-src', values: ["'none'"] },
  ],
  'Allow Inline Styles': [
    { directive: 'default-src', values: ["'self'"] },
    { directive: 'script-src', values: ["'self'"] },
    { directive: 'style-src', values: ["'self'", "'unsafe-inline'"] },
    { directive: 'img-src', values: ["'self'", 'data:', 'https:'] },
    { directive: 'font-src', values: ["'self'", 'data:'] },
  ],
  'Development Mode': [
    { directive: 'default-src', values: ["'self'"] },
    { directive: 'script-src', values: ["'self'", "'unsafe-inline'", "'unsafe-eval'"] },
    { directive: 'style-src', values: ["'self'", "'unsafe-inline'"] },
    { directive: 'img-src', values: ["'self'", 'data:', 'https:', 'http:'] },
  ],
};

export function CSPBuilder({ value, onChange, onValidate }: CSPBuilderProps) {
  const [directives, setDirectives] = useState<CSPDirective[]>([]);
  const [newDirective, setNewDirective] = useState('default-src');
  const [newValue, setNewValue] = useState('');
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [showPreview, setShowPreview] = useState(false);

  // Parse initial value
  useEffect(() => {
    try {
      if (value) {
        const parsed = JSON.parse(value) as CSPDirective[];
        setDirectives(parsed);
      } else {
        setDirectives([]);
      }
    } catch {
      setDirectives([]);
    }
  }, [value]);

  // Generate CSP string preview
  const generateCSPString = (dirs: CSPDirective[]): string => {
    return dirs
      .map((dir) => `${dir.directive} ${dir.values.join(' ')}`)
      .join('; ');
  };

  const cspString = generateCSPString(directives);

  // Update parent component
  const updateDirectives = (newDirectives: CSPDirective[]) => {
    setDirectives(newDirectives);
    onChange(JSON.stringify(newDirectives));
    validateCSP(newDirectives);
  };

  const validateCSP = (dirs: CSPDirective[]) => {
    const errors: string[] = [];

    // Check for duplicate directives
    const directiveNames = dirs.map((d) => d.directive);
    const duplicates = directiveNames.filter((name, index) => directiveNames.indexOf(name) !== index);
    if (duplicates.length > 0) {
      errors.push(`Duplicate directives found: ${duplicates.join(', ')}`);
    }

    // Check for dangerous combinations
    const hasUnsafeInline = dirs.some((d) =>
      d.values.some((v) => v === "'unsafe-inline'" || v === "'unsafe-eval'")
    );
    if (hasUnsafeInline) {
      errors.push('Using unsafe-inline or unsafe-eval weakens CSP protection');
    }

    // Check if default-src is set
    const hasDefaultSrc = dirs.some((d) => d.directive === 'default-src');
    if (!hasDefaultSrc && dirs.length > 0) {
      errors.push('Consider setting default-src as a fallback for all directives');
    }

    setValidationErrors(errors);
    onValidate?.(errors.length === 0, errors);
  };

  const handleAddDirective = () => {
    if (!newValue.trim()) return;

    const existingIndex = directives.findIndex((d) => d.directive === newDirective);

    let updated: CSPDirective[];
    if (existingIndex >= 0) {
      // Add to existing directive
      const existing = directives[existingIndex];
      if (!existing.values.includes(newValue.trim())) {
        const updatedDirective = {
          ...existing,
          values: [...existing.values, newValue.trim()],
        };
        updated = [
          ...directives.slice(0, existingIndex),
          updatedDirective,
          ...directives.slice(existingIndex + 1),
        ];
      } else {
        return; // Value already exists
      }
    } else {
      // Create new directive
      updated = [...directives, { directive: newDirective, values: [newValue.trim()] }];
    }

    updateDirectives(updated);
    setNewValue('');
  };

  const handleRemoveDirective = (directive: string) => {
    updateDirectives(directives.filter((d) => d.directive !== directive));
  };

  const handleRemoveValue = (directive: string, value: string) => {
    updateDirectives(
      directives.map((d) =>
        d.directive === directive
          ? { ...d, values: d.values.filter((v) => v !== value) }
          : d
      ).filter((d) => d.values.length > 0)
    );
  };

  const handleApplyPreset = (presetName: string) => {
    const preset = CSP_PRESETS[presetName];
    if (preset) {
      updateDirectives(preset);
    }
  };

  return (
    <Card className="p-4 space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold text-gray-900 dark:text-white">Content Security Policy Builder</h3>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setShowPreview(!showPreview)}
        >
          <Code className="w-4 h-4 mr-2" />
          {showPreview ? 'Hide' : 'Show'} Preview
        </Button>
      </div>

      {/* Preset Buttons */}
      <div className="flex flex-wrap gap-2">
        <span className="text-sm text-gray-600 dark:text-gray-400 self-center">Quick Presets:</span>
        {Object.keys(CSP_PRESETS).map((presetName) => (
          <Button
            key={presetName}
            variant="outline"
            size="sm"
            onClick={() => handleApplyPreset(presetName)}
          >
            {presetName}
          </Button>
        ))}
      </div>

      {/* Add Directive Form */}
      <div className="flex gap-2">
        <NativeSelect
          value={newDirective}
          onChange={(e) => setNewDirective(e.target.value)}
          className="w-48"
        >
          {CSP_DIRECTIVES.map((dir) => (
            <option key={dir} value={dir}>
              {dir}
            </option>
          ))}
        </NativeSelect>

        <div className="flex-1 flex gap-2">
          <Input
            type="text"
            value={newValue}
            onChange={(e) => setNewValue(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleAddDirective()}
            placeholder="Enter value or select from suggestions..."
            list="csp-values"
          />
          <datalist id="csp-values">
            {CSP_VALUES.map((val) => (
              <option key={val} value={val} />
            ))}
          </datalist>
          <Button onClick={handleAddDirective} disabled={!newValue.trim()}>
            <Plus className="w-4 h-4" />
          </Button>
        </div>
      </div>

      {/* Current Directives */}
      <div className="space-y-2">
        {directives.length === 0 ? (
          <Alert variant="info">
            <AlertCircle className="w-4 h-4" />
            <span>No CSP directives configured. Add directives above to build your policy.</span>
          </Alert>
        ) : (
          directives.map((dir) => (
            <div key={dir.directive} className="flex items-start gap-2 p-3 bg-gray-50 dark:bg-gray-800 rounded-lg">
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-2">
                  <span className="font-mono text-sm font-semibold text-gray-900 dark:text-white">
                    {dir.directive}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleRemoveDirective(dir.directive)}
                    className="ml-auto"
                  >
                    <X className="w-4 h-4" />
                  </Button>
                </div>
                <div className="flex flex-wrap gap-1">
                  {dir.values.map((val) => (
                    <Badge
                      key={val}
                      variant="outline"
                      className="flex items-center gap-1 cursor-pointer hover:bg-gray-300 dark:hover:bg-gray-600"
                      onClick={() => handleRemoveValue(dir.directive, val)}
                    >
                      <span className="font-mono text-xs">{val}</span>
                      <X className="w-3 h-3" />
                    </Badge>
                  ))}
                </div>
              </div>
            </div>
          ))
        )}
      </div>

      {/* Validation Errors */}
      {validationErrors.length > 0 && (
        <Alert variant="warning">
          <AlertCircle className="w-4 h-4" />
          <div>
            <p className="font-semibold mb-1">CSP Validation Warnings:</p>
            <ul className="list-disc list-inside text-sm space-y-1">
              {validationErrors.map((error, index) => (
                <li key={index}>{error}</li>
              ))}
            </ul>
          </div>
        </Alert>
      )}

      {validationErrors.length === 0 && directives.length > 0 && (
        <Alert variant="success">
          <Check className="w-4 h-4" />
          <span>CSP configuration looks good!</span>
        </Alert>
      )}

      {/* CSP String Preview */}
      {showPreview && cspString && (
        <div className="space-y-2">
          <label className="text-sm font-medium text-gray-700 dark:text-gray-300">Generated CSP Header:</label>
          <pre className="p-3 bg-gray-900 dark:bg-gray-950 text-green-400 rounded-lg overflow-x-auto text-xs font-mono">
            {cspString || '(empty)'}
          </pre>
        </div>
      )}
    </Card>
  );
}
