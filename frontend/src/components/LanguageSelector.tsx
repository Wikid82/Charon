import { Globe } from 'lucide-react'

import { type Language } from '../context/LanguageContextValue'
import { useLanguage } from '../hooks/useLanguage'

const languageOptions: { code: Language; label: string; nativeLabel: string }[] = [
  { code: 'en', label: 'English', nativeLabel: 'English' },
  { code: 'es', label: 'Spanish', nativeLabel: 'Español' },
  { code: 'fr', label: 'French', nativeLabel: 'Français' },
  { code: 'de', label: 'German', nativeLabel: 'Deutsch' },
  { code: 'zh', label: 'Chinese', nativeLabel: '中文' },
]

export function LanguageSelector() {
  const { language, setLanguage } = useLanguage()

  const handleChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    setLanguage(e.target.value as Language)
  }

  return (
    <div className="flex items-center gap-3">
      <Globe className="h-5 w-5 text-content-secondary" />
      <select
        id="language-selector"
        data-testid="language-selector"
        aria-label="Language"
        value={language}
        onChange={handleChange}
        className="bg-surface-elevated border border-border rounded-md px-3 py-2 text-content-primary focus:outline-none focus:ring-2 focus:ring-primary focus:border-transparent transition-all"
      >
        {languageOptions.map((option) => (
          <option key={option.code} value={option.code}>
            {option.nativeLabel}
          </option>
        ))}
      </select>
    </div>
  )
}
