<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('authmail.verify.pending')} — NOXIOAI` })

const state = ref<'pending' | 'success' | 'error'>('pending')

onMounted(async () => {
  const token = route.query.token as string | undefined
  if (!token) {
    state.value = 'error'
    return
  }
  try {
    await $fetch(`${api}/api/auth/verify/confirm`, {
      method: 'POST', credentials: 'include',
      body: { token }
    })
    state.value = 'success'
  } catch {
    state.value = 'error'
  }
})
</script>

<template>
  <main class="auth-wrap min-h-dvh flex items-center justify-center px-6 py-16">
    <div class="site-grid" aria-hidden="true" />
    <div class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-10 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center justify-center gap-2 text-lg font-extrabold tracking-tight mb-8"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>

      <template v-if="state === 'pending'">
        <p class="text-dim">{{ $t('authmail.verify.pending') }}</p>
      </template>

      <template v-else-if="state === 'success'">
        <span class="status-icon status-icon--gold mx-auto" aria-hidden="true">✓</span>
        <h1 class="mt-5 text-2xl font-bold">{{ $t('authmail.verify.success') }}</h1>
        <p class="mt-3 text-sm text-dim">{{ $t('authmail.verify.successDesc') }}</p>
        <NuxtLink :to="localePath('/login')" class="mt-7 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep">
          {{ $t('authmail.verify.goToLogin') }}
        </NuxtLink>
      </template>

      <template v-else>
        <span class="status-icon status-icon--danger mx-auto" aria-hidden="true">✕</span>
        <h1 class="mt-5 text-2xl font-bold text-red-400">{{ $t('authmail.verify.error') }}</h1>
        <p class="mt-3 text-sm text-dim">{{ $t('authmail.verify.errorDesc') }}</p>
        <NuxtLink :to="localePath('/login')" class="mt-7 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep">
          {{ $t('authmail.verify.goToLogin') }}
        </NuxtLink>
      </template>
    </div>
  </main>
</template>

<style scoped>
.status-icon {
  align-items: center;
  border-radius: 999px;
  display: flex;
  font-size: 1.25rem;
  font-weight: 700;
  inline-size: 2.75rem;
  block-size: 2.75rem;
  justify-content: center;
}
.status-icon--gold {
  background: rgba(212, 191, 148, 0.14);
  border: 1px solid var(--gold);
  color: var(--gold);
}
.status-icon--danger {
  background: rgba(248, 113, 113, 0.1);
  border: 1px solid rgba(248, 113, 113, 0.5);
  color: #f87171;
}
</style>
