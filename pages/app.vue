<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('auth.app.greeting')} — NOXIOAI` })

const me = ref<{ name?: string; email?: string } | null>(null)

onMounted(async () => {
  try {
    me.value = await $fetch(`${api}/api/auth/me`, { credentials: 'include' })
  } catch {
    await navigateTo(localePath('/login'))
  }
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

      <h1 class="mt-10 text-3xl font-extrabold">
        <span class="text-gradient">{{ $t('auth.app.greeting') }}</span>
        <span v-if="me?.name">, {{ me.name }}</span>
      </h1>
      <p class="mt-2 text-dim">{{ $t('auth.app.subtitle') }}</p>

      <div class="mt-10 grid gap-6 sm:grid-cols-3">
        <div v-for="c in cards" :key="c.key" class="glass-card coming-soon rounded-2xl p-8">
          <div class="text-2xl text-gold">{{ c.icon }}</div>
          <h2 class="mt-4 font-bold">{{ $t(`auth.app.${c.key}`) }}</h2>
          <p class="mt-1.5 text-sm text-dim">{{ $t('auth.app.soon') }}</p>
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
.coming-soon {
  opacity: 0.88;
}
</style>
