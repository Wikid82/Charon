# Contributing Translations

Thank you for your interest in translating Charon! This guide will help you contribute translations in your language.

## Overview

Charon uses [i18next](https://www.i18next.com/) and [react-i18next](https://react.i18next.com/) for internationalization (i18n). All translations are stored in JSON files organized by language.

## Supported Languages

Currently, Charon supports the following languages:

- 🇬🇧 English (`en`) - Default
- 🇪🇸 Spanish (`es`)
- 🇫🇷 French (`fr`)
- 🇩🇪 German (`de`)
- 🇨🇳 Chinese (`zh`)

## File Structure

Translation files are located in `frontend/src/locales/`:

```plaintext
frontend/src/locales/
├── en/
│   └── translation.json  (Base translation - always up to date)
├── es/
│   └── translation.json
├── fr/
│   └── translation.json
├── de/
│   └── translation.json
└── zh/
    └── translation.json
```

## How to Contribute

### Adding a New Language

1. **Create a new language directory** in `frontend/src/locales/` with the ISO 639-1 language code (e.g., `pt` for Portuguese)

2. **Copy the English translation file** as a starting point:

   ```bash
   cp frontend/src/locales/en/translation.json frontend/src/locales/pt/translation.json
   ```

3. **Translate all strings** in the new file, keeping the JSON structure intact

4. **Update the i18n configuration** in `frontend/src/i18n.ts`:

   ```typescript
   import ptTranslation from './locales/pt/translation.json'

   const resources = {
     en: { translation: enTranslation },
     es: { translation: esTranslation },
     // ... other languages
     pt: { translation: ptTranslation }, // Add your new language
   }
   ```

5. **Update the Language type** in `frontend/src/context/LanguageContextValue.ts`:

   ```typescript
   export type Language = 'en' | 'es' | 'fr' | 'de' | 'zh' | 'pt' // Add new language
   ```

6. **Update the LanguageSelector component** in `frontend/src/components/LanguageSelector.tsx`:

   ```typescript
   const languageOptions: { code: Language; label: string; nativeLabel: string }[] = [
     // ... existing languages
     { code: 'pt', label: 'Portuguese', nativeLabel: 'Português' },
   ]
   ```

7. **Test your translation** by running the application and selecting your language

8. **Submit a pull request** with your changes

### Improving Existing Translations

1. **Find the translation file** for your language in `frontend/src/locales/{language-code}/translation.json`

2. **Make your improvements**, ensuring you maintain the JSON structure

3. **Test the changes** by running the application

4. **Submit a pull request** with a clear description of your improvements

## Translation Guidelines

### General Rules

1. **Preserve placeholders**: Keep interpolation variables like `{{count}}` intact
   - ✅ `"activeHosts": "{{count}} activo"`
   - ❌ `"activeHosts": "5 activo"`

2. **Maintain JSON structure**: Don't add or remove keys, only translate values
   - ✅ Keep all keys exactly as they appear in the English file
   - ❌ Don't rename keys or change nesting

3. **Use native language**: Translate to what native speakers would naturally say
   - ✅ "Configuración" (Spanish for Settings)
   - ❌ "Settings" (leaving it in English)

4. **Keep formatting consistent**: Respect capitalization and punctuation conventions of your language

5. **Test your translations**: Always verify your translations in the application to ensure they fit in the UI

### Translation Keys

The translation file is organized into logical sections:

- **`common`**: Frequently used UI elements (buttons, labels, actions)
- **`navigation`**: Menu and navigation items
- **`dashboard`**: Dashboard-specific strings
- **`settings`**: Settings page strings
- **`proxyHosts`**: Proxy hosts page strings
- **`certificates`**: Certificate management strings
- **`auth`**: Authentication and login strings
- **`errors`**: Error messages
- **`notifications`**: Success/failure notifications

### Example Translation

Here's an example of translating a section from English to Spanish:

```json
// English (en/translation.json)
{
  "common": {
    "save": "Save",
    "cancel": "Cancel",
    "delete": "Delete"
  }
}

// Spanish (es/translation.json)
{
  "common": {
    "save": "Guardar",
    "cancel": "Cancelar",
    "delete": "Eliminar"
  }
}
```

## Testing Translations

### Manual Testing

1. Start the development server:

   ```bash
   cd frontend
   npm run dev
   ```

2. Open the application in your browser (usually `http://localhost:5173`)

3. Navigate to **Settings** → **System** → **Language**

4. Select your language from the dropdown

5. Navigate through the application to verify all translations appear correctly

### Automated Testing

Run the i18n tests to verify your translations:

```bash
cd frontend
npm test -- src/__tests__/i18n.test.ts
```

## Building the Application

Before submitting your PR, ensure the application builds successfully:

```bash
cd frontend
npm run build
```

## RTL (Right-to-Left) Languages

If you're adding a Right-to-Left language (e.g., Arabic, Hebrew):

1. Add the language code to the RTL check in `frontend/src/context/LanguageContext.tsx`
2. Test the UI thoroughly to ensure proper RTL layout
3. You may need to update CSS for proper RTL support

## Questions or Issues?

If you have questions or run into issues while contributing translations:

1. Open an issue on GitHub with the `translation` label
2. Describe your question or problem clearly
3. Include the language you're working on

## Translation Status

To check which translations need updates, compare your language file with the English (`en/translation.json`) file. Any keys present in English but missing in your language file should be added.

## Thank You

Your contributions help make Charon accessible to users worldwide. Thank you for taking the time to improve the internationalization of this project!
