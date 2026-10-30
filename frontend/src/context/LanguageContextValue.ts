import { createContext } from 'react'

export type Language =
  | 'en'
  | 'es'
  | 'fr'
  | 'de'
  | 'zh'
  | 'af'
  | 'ar'
  | 'ca'
  | 'cs'
  | 'da'
  | 'el'
  | 'fi'
  | 'he'
  | 'hu'
  | 'it'
  | 'ja'
  | 'ko'
  | 'nl'
  | 'no'
  | 'pl'
  | 'pt'
  | 'ro'
  | 'ru'
  | 'sr'
  | 'sv'
  | 'tr'
  | 'uk'
  | 'vi'

export interface LanguageContextType {
  language: Language
  setLanguage: (lang: Language) => void
}

export const LanguageContext = createContext<LanguageContextType | undefined>(undefined)
