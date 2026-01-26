import { ReactNode, useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { LanguageContext, Language } from './LanguageContextValue'

export function LanguageProvider({ children }: { children: ReactNode }) {
  const { i18n } = useTranslation()
  const [language, setLanguageState] = useState<Language>(() => {
    const saved = localStorage.getItem('charon-language')
    return (saved as Language) || 'en'
  })

  useEffect(() => {
    i18n.changeLanguage(language)
  }, [language, i18n])

  const setLanguage = (lang: Language) => {
    setLanguageState(lang)
    localStorage.setItem('charon-language', lang)
    i18n.changeLanguage(lang)
    // Set document direction for RTL languages
    // Currently only LTR languages are supported (en, es, fr, de, zh)
    // When adding RTL languages (ar, he), update the Language type and this check:
    // document.documentElement.dir = ['ar', 'he'].includes(lang) ? 'rtl' : 'ltr'
    document.documentElement.dir = 'ltr'
  }

  return (
    <LanguageContext.Provider value={{ language, setLanguage }}>
      {children}
    </LanguageContext.Provider>
  )
}
