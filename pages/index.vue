<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()

const faqKeys = ['one', 'two', 'three', 'four'] as const
const faqPageUrl = computed(() => new URL(localePath('/'), 'https://noxioai.com').toString().replace(/\/$/, ''))
const faqSchema = computed(() => ({
  '@context': 'https://schema.org',
  '@type': 'FAQPage',
  '@id': `${faqPageUrl.value}#faq`,
  mainEntity: faqKeys.map(key => ({
    '@type': 'Question',
    name: t(`faq.items.${key}.q`),
    acceptedAnswer: {
      '@type': 'Answer',
      text: t(`faq.items.${key}.a`)
    }
  }))
}))

useSeoMeta({
  title: () => t('meta.title'),
  description: () => t('meta.description'),
  ogTitle: () => t('meta.title'),
  ogDescription: () => t('meta.description'),
  ogType: 'website'
})

useHead(() => ({
  script: [
    {
      key: 'schema-org-faq',
      id: 'schema-org-faq',
      type: 'application/ld+json',
      innerHTML: JSON.stringify(faqSchema.value)
    }
  ]
}))
</script>

<template>
  <main class="noxio-home">
    <div class="site-grid" aria-hidden="true" />
    <div class="site-glow site-glow--one" aria-hidden="true" />
    <div class="site-glow site-glow--two" aria-hidden="true" />
    <LandingHero />
    <LandingShowcase />
    <LandingTeam />
    <LandingFeatures />
    <LandingHow />
    <LandingPricing />
    <LandingFaq />
    <LandingWaitlist />
    <LandingFooter />
  </main>
</template>
