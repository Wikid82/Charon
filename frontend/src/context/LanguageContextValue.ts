import { createContext } from 'react'

export type Language = 'en' | 'es' | 'fr' | 'de' | 'zh'

export interface LanguageContextType {
  language: Language
  setLanguage: (lang: Language) => void
}

export const LanguageContext = createContext<LanguageContextType | undefined>(undefined)
