export default defineNuxtConfig({
  compatibilityDate: '2026-07-05',
  modules: ['@nuxtjs/tailwindcss', '@nuxtjs/i18n', '@vueuse/motion/nuxt'],

  css: ['~/assets/css/main.css'],

  runtimeConfig: {
    public: {
      // set NUXT_PUBLIC_WEB3FORMS_KEY in the deploy environment; empty = form shows email fallback
      web3formsKey: ''
    }
  },

  i18n: {
    locales: [
      { code: 'fa', language: 'fa-IR', name: 'فارسی', dir: 'rtl', file: 'fa.json' },
      { code: 'en', language: 'en-US', name: 'English', dir: 'ltr', file: 'en.json' },
      { code: 'tr', language: 'tr-TR', name: 'Türkçe', dir: 'ltr', file: 'tr.json' },
      { code: 'ar', language: 'ar', name: 'العربية', dir: 'rtl', file: 'ar.json' }
    ],
    defaultLocale: 'fa',
    strategy: 'prefix_except_default',
    lazy: true,
    baseUrl: 'https://noxioai.com',
    detectBrowserLanguage: false
  },

  app: {
    head: {
      link: [
        { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
        { rel: 'preconnect', href: 'https://fonts.googleapis.com' },
        { rel: 'preconnect', href: 'https://fonts.gstatic.com', crossorigin: '' },
        {
          rel: 'stylesheet',
          href: 'https://fonts.googleapis.com/css2?family=Inter:wght@400;600;800&family=Vazirmatn:wght@400;600;800&display=swap'
        }
      ]
    }
  }
})
