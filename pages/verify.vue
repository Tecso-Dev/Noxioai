<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const api = useRuntimeConfig().public.apiBase

useSeoMeta({ title: () => `${t('authmail.verify.pending')} — NOXIOAI` })
const state = ref<'pending' | 'success' | 'error'>('pending')

onMounted(async () => {
  const token = typeof route.query.token === 'string' ? route.query.token : ''
  if (!token) {
    state.value = 'error'
    return
  }
  try {
    await $fetch(`${api}/api/auth/verify/confirm`, {
      method: 'POST', credentials: 'include', body: { token }
    })
    state.value = 'success'
  } catch {
    state.value = 'error'
  }
})

const title = computed(() => state.value === 'success' ? t('authmail.verify.success') : state.value === 'error' ? t('authmail.verify.error') : t('authmail.verify.pending'))
const description = computed(() => state.value === 'success' ? t('authmail.verify.successDesc') : state.value === 'error' ? t('authmail.verify.errorDesc') : t('authmail.verify.pendingDesc'))
</script>

<template>
  <AuthShell :eyebrow="$t('authmail.verify.eyebrow')" :title="title" :description="description">
    <div class="auth-complete" role="status" aria-live="polite">
      <span v-if="state === 'pending'" class="verify-loader" aria-hidden="true" />
      <span v-else class="auth-complete__icon" :class="{ 'verify-error': state === 'error' }" aria-hidden="true">
        <svg viewBox="0 0 24 24"><path :d="state === 'success' ? 'm5 12 4 4L19 6' : 'M7 7l10 10M17 7 7 17'"/></svg>
      </span>
      <NuxtLink v-if="state !== 'pending'" :to="localePath('/login')" class="auth-primary">{{ $t('authmail.verify.goToLogin') }}</NuxtLink>
    </div>
  </AuthShell>
</template>

<style scoped>
.verify-loader { animation: verify-spin .8s linear infinite; border: 2px solid rgba(72, 202, 228, .16); border-block-start-color: var(--cyan); border-radius: 50%; block-size: 2.8rem; inline-size: 2.8rem; }
.verify-error { background: rgba(248, 113, 113, .08); border-color: rgba(248, 113, 113, .35); color: #f87171; }
@keyframes verify-spin { to { rotate: 360deg; } }
@media (prefers-reduced-motion: reduce) { .verify-loader { animation-duration: 2s; } }
</style>
