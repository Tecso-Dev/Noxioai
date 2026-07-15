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
    <div class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-8 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark block text-lg font-extrabold tracking-tight mb-6">NOXIO<span class="text-brand2">AI</span></NuxtLink>

      <template v-if="state === 'pending'">
        <p class="text-dim">{{ $t('authmail.verify.pending') }}</p>
      </template>

      <template v-else-if="state === 'success'">
        <h1 class="text-2xl font-bold">{{ $t('authmail.verify.success') }}</h1>
        <p class="mt-3 text-sm text-dim">{{ $t('authmail.verify.successDesc') }}</p>
        <NuxtLink :to="localePath('/login')" class="mt-6 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition">
          {{ $t('authmail.verify.goToLogin') }}
        </NuxtLink>
      </template>

      <template v-else>
        <h1 class="text-2xl font-bold">{{ $t('authmail.verify.error') }}</h1>
        <p class="mt-3 text-sm text-dim">{{ $t('authmail.verify.errorDesc') }}</p>
        <NuxtLink :to="localePath('/login')" class="mt-6 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition">
          {{ $t('authmail.verify.goToLogin') }}
        </NuxtLink>
      </template>
    </div>
  </main>
</template>
