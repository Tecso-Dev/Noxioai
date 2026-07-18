<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('auth.app.greeting')} — NOXIOAI` })

const me = ref<{ name?: string; email?: string; verified: boolean } | null>(null)
const loading = ref(true)
const hasProfile = ref<boolean | null>(null)

onMounted(async () => {
  try {
    me.value = await $fetch(`${api}/api/auth/me`, { credentials: 'include' })
  } catch {
    await navigateTo(localePath('/login'))
    return
  }

  if (me.value?.verified) {
    try {
      const profile = await $fetch<Record<string, unknown>>(`${api}/api/profile`, { credentials: 'include' })
      hasProfile.value = Object.keys(profile).length > 0
    } catch {
      hasProfile.value = null
    }
  }
  loading.value = false
})
const cards = [
  { key: 'leads', icon: '◎' },
  { key: 'outreach', icon: '✎' },
  { key: 'billing', icon: '❖' }
]
</script>

<template>
  <main class="app-wrap relative min-h-dvh px-6 py-16">
    <div class="site-grid" aria-hidden="true" />
    <div class="relative z-10 mx-auto max-w-4xl">
      <div class="flex items-center justify-between border-b border-line pb-6">
        <NuxtLink :to="localePath('/')" class="brand-mark flex items-center gap-2 text-lg font-extrabold tracking-tight"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
        <NuxtLink :to="localePath('/account')" class="text-sm text-dim transition hover:text-gold">{{ $t('auth.account.title') }}</NuxtLink>
      </div>

      <div v-if="loading" class="py-24 text-center text-dim" aria-live="polite">…</div>

      <section v-else-if="me?.verified === false" class="glass-card mx-auto mt-16 max-w-xl rounded-3xl border-t-2 border-t-gold p-8 text-center sm:p-12">
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full border border-gold/40 bg-gold/10 text-xl text-gold" aria-hidden="true">✉</div>
        <h1 class="mt-6 text-2xl font-extrabold">{{ $t('onboarding.verifyGateTitle') }}</h1>
        <p class="mx-auto mt-3 max-w-md text-dim">{{ $t('onboarding.verifyGateBody') }}</p>
      </section>

      <template v-else>
        <h1 class="mt-10 text-3xl font-extrabold">
          <span class="text-gradient">{{ $t('auth.app.greeting') }}</span>
          <span v-if="me?.name">, {{ me.name }}</span>
        </h1>
        <p class="mt-2 text-dim">{{ $t('auth.app.subtitle') }}</p>

        <section v-if="hasProfile === false" class="glass-card mt-8 rounded-2xl border border-gold/40 p-6 sm:flex sm:items-center sm:justify-between sm:gap-8 sm:p-8">
          <div>
            <h2 class="text-xl font-extrabold text-gold">{{ $t('onboarding.setupTitle') }}</h2>
            <p class="mt-2 text-sm text-dim">{{ $t('onboarding.setupDescription') }}</p>
          </div>
          <NuxtLink :to="localePath('/onboarding')" class="mt-5 inline-flex shrink-0 rounded-full bg-brand px-6 py-3 font-bold text-night transition hover:bg-gold-deep sm:mt-0">
            {{ $t('onboarding.setupCta') }}
          </NuxtLink>
        </section>

        <div v-else-if="hasProfile" class="mt-6 text-end">
          <NuxtLink :to="localePath('/onboarding')" class="text-sm font-semibold text-gold transition hover:text-gold-deep">
            {{ $t('onboarding.editProfile') }}
          </NuxtLink>
        </div>

        <div class="mt-10 grid gap-6 sm:grid-cols-3">
          <div v-for="c in cards" :key="c.key" class="glass-card coming-soon rounded-2xl p-8">
            <div class="text-2xl text-gold">{{ c.icon }}</div>
            <h2 class="mt-4 font-bold">{{ $t(`auth.app.${c.key}`) }}</h2>
            <p class="mt-1.5 text-sm text-dim">{{ $t('auth.app.soon') }}</p>
          </div>
        </div>
      </template>
    </div>
  </main>
</template>

<style scoped>
.coming-soon {
  opacity: 0.88;
}
</style>
