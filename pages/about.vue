<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()

useSeoMeta({
  title: () => t('about.meta.title'),
  description: () => t('about.meta.description'),
  ogTitle: () => t('about.meta.title'),
  ogDescription: () => t('about.meta.description'),
  ogType: 'website'
})
</script>

<template>
  <main class="services-page relative overflow-hidden">
    <div class="site-grid" aria-hidden="true" />
    <div class="site-glow site-glow--one" aria-hidden="true" />
    <div class="site-glow site-glow--two" aria-hidden="true" />

    <nav class="relative z-10 mx-auto flex max-w-6xl items-center justify-between px-6 py-6">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center gap-2 text-xl font-extrabold tracking-tight">
        <img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span>
      </NuxtLink>
      <LandingLangSwitcher />
    </nav>

    <header class="relative z-10 mx-auto max-w-3xl px-6 pb-10 pt-16 text-center sm:pt-24">
      <p class="text-sm font-semibold uppercase text-dim">{{ t('about.eyebrow') }}</p>
      <h1 class="mt-4 text-4xl font-extrabold leading-tight sm:text-6xl">
        <span class="text-gradient">{{ t('about.title') }}</span>
      </h1>
      <p class="mx-auto mt-6 max-w-prose text-xl font-semibold leading-relaxed text-ivory">{{ t('about.intro') }}</p>
    </header>

    <section class="relative z-10 mx-auto max-w-3xl px-6 pb-24 pt-8">
      <article class="glass-card rounded-3xl p-8 sm:p-12">
        <div class="about-section pb-8">
          <h2 class="text-2xl font-bold tracking-tight text-ivory">{{ t('about.story.title') }}</h2>
          <p class="mt-3 max-w-prose leading-relaxed text-dim">{{ t('about.story.body') }}</p>
        </div>
        <div class="about-section py-8">
          <h2 class="text-2xl font-bold tracking-tight text-ivory">{{ t('about.founder.title') }}</h2>
          <p class="mt-3 max-w-prose leading-relaxed text-dim">{{ t('about.founder.body') }}</p>
        </div>
        <div class="about-section pt-8">
          <h2 class="text-2xl font-bold tracking-tight text-ivory">{{ t('about.mission.title') }}</h2>
          <p class="mt-3 max-w-prose leading-relaxed text-dim">{{ t('about.mission.body') }}</p>
        </div>

        <NuxtLink
          :to="`${localePath('/')}#waitlist`"
          class="mt-10 inline-flex rounded-full bg-brand px-6 py-3 font-bold transition hover:brightness-105"
        >
          {{ t('about.cta') }}
        </NuxtLink>
      </article>
    </section>

    <LandingFooter />
  </main>
</template>

<style scoped>
.about-section + .about-section {
  border-block-start: 1px solid var(--line-soft);
}
</style>
