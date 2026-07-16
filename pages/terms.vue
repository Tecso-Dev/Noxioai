<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()

useSeoMeta({
  title: () => t('terms.meta.title'),
  description: () => t('terms.meta.description'),
  ogTitle: () => t('terms.meta.title'),
  ogDescription: () => t('terms.meta.description'),
  ogType: 'website'
})

const sections = [
  { key: 'acceptance', items: [] },
  { key: 'service', items: ['software', 'services'] },
  { key: 'accounts', items: ['accurate', 'security', 'eligibility'] },
  { key: 'acceptable', items: ['law', 'abuse', 'security', 'rights'] },
  { key: 'payments', items: ['billing', 'renewal', 'cancel', 'refunds', 'taxes'] },
  { key: 'ownership', items: ['noxioai', 'customer'] },
  { key: 'availability', items: [] },
  { key: 'liability', items: [] },
  { key: 'termination', items: [] },
  { key: 'law', items: [] },
  { key: 'changes', items: [] },
  { key: 'contact', items: [] }
] as const
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

    <header class="relative z-10 mx-auto max-w-3xl px-6 pb-8 pt-14 text-center sm:pt-20">
      <p class="text-sm font-semibold uppercase text-dim">{{ t('terms.eyebrow') }}</p>
      <h1 class="mt-4 text-4xl font-extrabold leading-tight sm:text-6xl">
        <span class="text-gradient">{{ t('terms.title') }}</span>
      </h1>
      <p class="mx-auto mt-6 max-w-prose text-lg leading-relaxed text-dim">{{ t('terms.intro') }}</p>
      <p class="mt-4 text-sm text-gold">{{ t('terms.updated') }}</p>
    </header>

    <section class="relative z-10 mx-auto max-w-3xl px-6 pb-24 pt-8">
      <article class="glass-card rounded-3xl p-7 sm:p-10">
        <section v-for="section in sections" :key="section.key" class="legal-section py-8 first:pt-0 last:pb-0">
          <h2 class="text-2xl font-bold tracking-tight text-ivory">{{ t(`terms.sections.${section.key}.title`) }}</h2>
          <p class="mt-3 max-w-prose leading-relaxed text-dim">{{ t(`terms.sections.${section.key}.body`) }}</p>

          <ul v-if="section.items.length" class="mt-6 space-y-5">
            <li v-for="item in section.items" :key="item" class="flex items-start gap-3">
              <span class="mt-1 text-sm text-gold" aria-hidden="true">✦</span>
              <div>
                <h3 class="font-semibold text-ivory">{{ t(`terms.sections.${section.key}.items.${item}.label`) }}</h3>
                <p class="mt-1 leading-relaxed text-dim">{{ t(`terms.sections.${section.key}.items.${item}.text`) }}</p>
              </div>
            </li>
          </ul>

          <a
            v-if="section.key === 'contact'"
            href="mailto:sobhan.gjhav.azimzadeh@gmail.com"
            class="mt-5 inline-block font-semibold text-gold transition hover:text-snow"
            dir="ltr"
          >sobhan.gjhav.azimzadeh&#64;gmail.com</a>
        </section>
      </article>
    </section>

    <LandingFooter />
  </main>
</template>

<style scoped>
.legal-section + .legal-section {
  border-block-start: 1px solid var(--line-soft);
}
</style>
