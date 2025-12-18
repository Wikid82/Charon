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
    // Future: Update document direction for RTL languages (e.g., Arabic, Hebrew)
    // Currently not used as we don't have RTL language translations yet
    document.documentElement.dir = 'ltr'
  }

  return (
    <LanguageContext.Provider value={{ language, setLanguage }}>
      {children}
    </LanguageContext.Provider>
  )
}
