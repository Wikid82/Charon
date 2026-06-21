/** @type {import('tailwindcss').Config} */
export default {
  darkMode: ['selector', '[data-theme="dark"]'],
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        // ========================================
        // BRAND COLORS
        // ========================================
        brand: {
          50: 'rgb(var(--color-brand-50) / <alpha-value>)',
          100: 'rgb(var(--color-brand-100) / <alpha-value>)',
          200: 'rgb(var(--color-brand-200) / <alpha-value>)',
          300: 'rgb(var(--color-brand-300) / <alpha-value>)',
          400: 'rgb(var(--color-brand-400) / <alpha-value>)',
          500: 'rgb(var(--color-brand-500) / <alpha-value>)',
          600: 'rgb(var(--color-brand-600) / <alpha-value>)',
          700: 'rgb(var(--color-brand-700) / <alpha-value>)',
          800: 'rgb(var(--color-brand-800) / <alpha-value>)',
          900: 'rgb(var(--color-brand-900) / <alpha-value>)',
          950: 'rgb(var(--color-brand-950) / <alpha-value>)',
        },
        // ========================================
        // SEMANTIC SURFACE COLORS
        // ========================================
        surface: {
          base: 'rgb(var(--color-bg-base) / <alpha-value>)',
          subtle: 'rgb(var(--color-bg-subtle) / <alpha-value>)',
          muted: 'rgb(var(--color-bg-muted) / <alpha-value>)',
          elevated: 'rgb(var(--color-bg-elevated) / <alpha-value>)',
          overlay: 'rgb(var(--color-bg-overlay) / <alpha-value>)',
        },
        // ========================================
        // SEMANTIC BORDER COLORS
        // ========================================
        border: {
          DEFAULT: 'rgb(var(--color-border-default) / <alpha-value>)',
          muted: 'rgb(var(--color-border-muted) / <alpha-value>)',
          strong: 'rgb(var(--color-border-strong) / <alpha-value>)',
        },
        // ========================================
        // SEMANTIC TEXT COLORS
        // ========================================
        content: {
          primary: 'rgb(var(--color-text-primary) / <alpha-value>)',
          secondary: 'rgb(var(--color-text-secondary) / <alpha-value>)',
          muted: 'rgb(var(--color-text-muted) / <alpha-value>)',
          inverted: 'rgb(var(--color-text-inverted) / <alpha-value>)',
        },
        // ========================================
        // STATE COLORS
        // ========================================
        success: {
          DEFAULT: 'rgb(var(--color-success) / <alpha-value>)',
          muted: 'rgb(var(--color-success-muted) / <alpha-value>)',
        },
        warning: {
          DEFAULT: 'rgb(var(--color-warning) / <alpha-value>)',
          muted: 'rgb(var(--color-warning-muted) / <alpha-value>)',
        },
        error: {
          DEFAULT: 'rgb(var(--color-error) / <alpha-value>)',
          muted: 'rgb(var(--color-error-muted) / <alpha-value>)',
        },
        info: {
          DEFAULT: 'rgb(var(--color-info) / <alpha-value>)',
          muted: 'rgb(var(--color-info-muted) / <alpha-value>)',
        },
        // ========================================
        // LEGACY COLORS (for backward compatibility)
        // ========================================
        'light-bg': '#f0f4f8',
        'dark-bg': '#0f172a',
        'dark-sidebar': '#020617',
        'dark-card': '#1e293b',
        'blue-active': '#1d4ed8',
        'blue-hover': '#2563eb',
      },
      // ========================================
      // FONT FAMILY
      // ========================================
      fontFamily: {
        sans: ['var(--font-sans)'],
        mono: ['var(--font-mono)'],
      },
      // ========================================
      // FONT SIZE WITH LINE HEIGHTS
      // ========================================
      fontSize: {
        xs: ['var(--text-xs)', { lineHeight: 'var(--leading-normal)' }],
        sm: ['var(--text-sm)', { lineHeight: 'var(--leading-normal)' }],
        base: ['var(--text-base)', { lineHeight: 'var(--leading-normal)' }],
        lg: ['var(--text-lg)', { lineHeight: 'var(--leading-normal)' }],
        xl: ['var(--text-xl)', { lineHeight: 'var(--leading-tight)' }],
        '2xl': ['var(--text-2xl)', { lineHeight: 'var(--leading-tight)' }],
        '3xl': ['var(--text-3xl)', { lineHeight: 'var(--leading-tight)' }],
        '4xl': ['var(--text-4xl)', { lineHeight: 'var(--leading-tight)' }],
      },
      // ========================================
      // BORDER RADIUS
      // ========================================
      borderRadius: {
        sm: 'var(--radius-sm)',
        DEFAULT: 'var(--radius-md)',
        md: 'var(--radius-md)',
        lg: 'var(--radius-lg)',
        xl: 'var(--radius-xl)',
        '2xl': 'var(--radius-2xl)',
      },
      // ========================================
      // BOX SHADOW
      // ========================================
      boxShadow: {
        sm: 'var(--shadow-sm)',
        DEFAULT: 'var(--shadow-md)',
        md: 'var(--shadow-md)',
        lg: 'var(--shadow-lg)',
        xl: 'var(--shadow-xl)',
      },
      // ========================================
      // TRANSITION DURATION
      // ========================================
      transitionDuration: {
        fast: 'var(--transition-fast)',
        normal: 'var(--transition-normal)',
        slow: 'var(--transition-slow)',
      },
      // ========================================
      // CUSTOM SPACING
      // ========================================
      spacing: {
        'page': 'var(--page-gutter)',
        'page-lg': 'var(--page-gutter-lg)',
      },
    },
  },
  plugins: [],
}
