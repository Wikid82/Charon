import { Globe } from 'lucide-react'

import { type Language } from '../context/LanguageContextValue'
import { useLanguage } from '../hooks/useLanguage'

const languageOptions: { code: Language; label: string; nativeLabel: string }[] = [
  { code: 'en', label: 'English', nativeLabel: 'English' },
  { code: 'es', label: 'Spanish', nativeLabel: 'Español' },
  { code: 'fr', label: 'French', nativeLabel: 'Français' },
  { code: 'de', label: 'German', nativeLabel: 'Deutsch' },
  { code: 'zh', label: 'Chinese', nativeLabel: '中文' },
  { code: 'af', label: 'Afrikaans', nativeLabel: 'Afrikaans' },
  { code: 'ar', label: 'Arabic', nativeLabel: 'العربية' },
  { code: 'ca', label: 'Catalan', nativeLabel: 'Català' },
  { code: 'cs', label: 'Czech', nativeLabel: 'Čeština' },
  { code: 'da', label: 'Danish', nativeLabel: 'Dansk' },
  { code: 'el', label: 'Greek', nativeLabel: 'Ελληνικά' },
  { code: 'fi', label: 'Finnish', nativeLabel: 'Suomi' },
  { code: 'he', label: 'Hebrew', nativeLabel: 'עברית' },
  { code: 'hu', label: 'Hungarian', nativeLabel: 'Magyar' },
  { code: 'it', label: 'Italian', nativeLabel: 'Italiano' },
  { code: 'ja', label: 'Japanese', nativeLabel: '日本語' },
  { code: 'ko', label: 'Korean', nativeLabel: '한국어' },
  { code: 'nl', label: 'Dutch', nativeLabel: 'Nederlands' },
  { code: 'no', label: 'Norwegian', nativeLabel: 'Norsk' },
  { code: 'pl', label: 'Polish', nativeLabel: 'Polski' },
  { code: 'pt', label: 'Portuguese', nativeLabel: 'Português' },
  { code: 'ro', label: 'Romanian', nativeLabel: 'Română' },
  { code: 'ru', label: 'Russian', nativeLabel: 'Русский' },
  { code: 'sr', label: 'Serbian', nativeLabel: 'Српски' },
  { code: 'sv', label: 'Swedish', nativeLabel: 'Svenska' },
  { code: 'tr', label: 'Turkish', nativeLabel: 'Türkçe' },
  { code: 'uk', label: 'Ukrainian', nativeLabel: 'Українська' },
  { code: 'vi', label: 'Vietnamese', nativeLabel: 'Tiếng Việt' },
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
