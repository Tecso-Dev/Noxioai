import type { Config } from 'tailwindcss'

export default {
  content: ['./components/**/*.vue', './pages/**/*.vue', './app.vue'],
  theme: {
    extend: {
      colors: {
        night: '#0b0b12',
        panel: '#14141f',
        line: '#262636',
        snow: '#f2efe8',
        ivory: '#f2efe8',
        gold: '#d4bf94',
        'gold-deep': '#b39868',
        dim: '#9a95b0',
        brand: '#d4bf94',
        brand2: '#48CAE4'
      },
      fontFamily: {
        sans: ['Vazirmatn', 'Inter', 'system-ui', 'sans-serif']
      }
    }
  }
} satisfies Config
