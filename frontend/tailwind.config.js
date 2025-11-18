/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      colors: {
        'dark-bg': '#0f172a',
        'dark-sidebar': '#020617',
        'dark-card': '#1e293b',
        'blue-active': '#1d4ed8',
        'blue-hover': '#2563eb',
      },
    },
  },
  plugins: [],
}
