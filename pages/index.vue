<script setup lang="ts">
const { t } = useI18n()

useSeoMeta({
  title: () => t('meta.title'),
  description: () => t('meta.description'),
  ogTitle: () => t('meta.title'),
  ogDescription: () => t('meta.description'),
  ogType: 'website'
})
</script>

<template>
  <main class="noxio-home">
    <div class="site-grid" aria-hidden="true" />
    <div class="site-glow site-glow--one" aria-hidden="true" />
    <div class="site-glow site-glow--two" aria-hidden="true" />
    <LandingHero />
    <LandingTeam />
    <LandingFeatures />
    <LandingHow />
    <LandingPricing />
    <LandingFaq />
    <LandingWaitlist />
    <footer class="noxio-footer relative border-t border-line py-8 text-center text-sm text-dim">
      <p>NOXIOAI — {{ $t('footer.soon') }}</p>
      <p class="mt-1">{{ $t('footer.made') }} · <a href="https://github.com/Tecso-Dev" class="hover:text-brand2 transition">Tecso-Dev</a></p>
    </footer>
  </main>
</template>
