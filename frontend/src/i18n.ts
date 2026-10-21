import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import resourcesToBackend from 'i18next-resources-to-backend'
import { initReactI18next } from 'react-i18next'

import enTranslation from './locales/en/translation.json'

// English is bundled because it is the fallback language; every other locale is
// split into its own chunk and fetched only when that language is selected.
const resources = {
  en: { translation: enTranslation },
}

const lazyLocales: Record<string, () => Promise<unknown>> = {
  de: () => import('./locales/de/translation.json'),
  es: () => import('./locales/es/translation.json'),
  fr: () => import('./locales/fr/translation.json'),
  zh: () => import('./locales/zh/translation.json'),
}

i18n
  .use(LanguageDetector) // Detect user language
  .use(resourcesToBackend((language: string) => lazyLocales[language]?.()))
  .use(initReactI18next) // Pass i18n instance to react-i18next
  .init({
    resources,
    partialBundledLanguages: true, // English is bundled; other languages load through the backend
    supportedLngs: ['en', ...Object.keys(lazyLocales)],
    fallbackLng: 'en', // Fallback to English if translation not found
    debug: false, // Debug mode disabled (enable temporarily for troubleshooting)
    interpolation: {
      escapeValue: false, // React already escapes values
    },
    detection: {
      order: ['localStorage', 'navigator'], // Check localStorage first, then browser language
      caches: ['localStorage'], // Cache language selection in localStorage
      lookupLocalStorage: 'charon-language', // Key for storing language in localStorage
    },
  })

export default i18n
