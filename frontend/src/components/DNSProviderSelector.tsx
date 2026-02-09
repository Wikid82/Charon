import { useTranslation } from 'react-i18next'
import { Star } from 'lucide-react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  Label,
} from './ui'
import { useDNSProviders } from '../hooks/useDNSProviders'

interface DNSProviderSelectorProps {
  value?: number
  onChange: (providerId: number | undefined) => void
  required?: boolean
  disabled?: boolean
  label?: string
  helperText?: string
  error?: string
}

export default function DNSProviderSelector({
  value,
  onChange,
  required = false,
  disabled = false,
  label,
  helperText,
  error,
}: DNSProviderSelectorProps) {
  const { t } = useTranslation()
  const { data: providers = [], isLoading } = useDNSProviders()

  // Filter to only enabled providers with credentials
  const availableProviders = providers.filter(
    (p) => p.enabled && p.has_credentials
  )

  const handleValueChange = (value: string) => {
    if (value === 'none') {
      onChange(undefined)
    } else {
      onChange(parseInt(value, 10))
    }
  }

  return (
    <div className="w-full">
      {label && (
        <Label className="block text-sm font-medium text-content-secondary mb-1.5">
          {label}
          {required && <span className="text-error ml-1">*</span>}
        </Label>
      )}
      <Select
        value={value ? value.toString() : 'none'}
        onValueChange={handleValueChange}
        disabled={disabled || isLoading}
      >
        <SelectTrigger error={!!error}>
          <SelectValue placeholder={t('dnsProviders.selectProvider')} />
        </SelectTrigger>
        <SelectContent>
          {!required && (
            <SelectItem value="none">
              {t('dnsProviders.noProvider')}
            </SelectItem>
          )}
          {isLoading && (
            <SelectItem value="loading" disabled>
              {t('common.loading')}
            </SelectItem>
          )}
          {!isLoading && availableProviders.length === 0 && (
            <SelectItem value="empty" disabled>
              {t('dnsProviders.noProvidersAvailable')}
            </SelectItem>
          )}
          {availableProviders.map((provider) => (
            <SelectItem key={provider.id} value={provider.id.toString()}>
              <div className="flex items-center gap-2">
                {provider.name}
                {provider.is_default && (
                  <Star className="w-3 h-3 text-yellow-500 fill-yellow-500" />
                )}
                <span className="text-content-muted text-xs">
                  ({t(`dnsProviders.types.${provider.provider_type}`, provider.provider_type)})
                </span>
              </div>
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {error && (
        <p className="mt-1.5 text-sm text-error" role="alert">
          {error}
        </p>
      )}
      {helperText && !error && (
        <p className="mt-1.5 text-sm text-content-muted">{helperText}</p>
      )}
    </div>
  )
}
