<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('authmail.reset.title')} — NOXIOAI` })

const token = computed(() => route.query.token as string | undefined)

// Request-link form state
const email = ref('')
const requestSent = ref(false)
const requestBusy = ref(false)

// New-password form state
const password = ref('')
const confirmState = ref<'idle' | 'success' | 'invalid'>('idle')
const confirmBusy = ref(false)
const err = ref('')

async function requestLink() {
  err.value = ''
  requestBusy.value = true
  try {
    await $fetch(`${api}/api/auth/reset/request`, {
      method: 'POST', credentials: 'include',
      body: { email: email.value }
    })
    requestSent.value = true
  } catch (e: any) {
    err.value = e?.data || t('auth.error')
  } finally {
    requestBusy.value = false
  }
}

async function confirmReset() {
  err.value = ''
  confirmBusy.value = true
  try {
    await $fetch(`${api}/api/auth/reset/confirm`, {
      method: 'POST', credentials: 'include',
      body: { token: token.value, password: password.value }
    })
    confirmState.value = 'success'
  } catch (e: any) {
    if (e?.status === 400) confirmState.value = 'invalid'
    else err.value = e?.data || t('auth.error')
  } finally {
    confirmBusy.value = false
  }
}
</script>

<template>
  <main class="auth-wrap min-h-dvh flex items-center justify-center px-6 py-16">
    <div class="site-grid" aria-hidden="true" />

    <!-- No token: request a reset link -->
    <form v-if="!token && !requestSent" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-10" @submit.prevent="requestLink">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center justify-center gap-2 text-lg font-extrabold tracking-tight mb-8"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold text-center">{{ $t('authmail.reset.title') }}</h1>
      <p class="mt-2 text-center text-sm text-dim">{{ $t('authmail.reset.desc') }}</p>

      <label class="block mt-8 text-sm text-dim">{{ $t('authmail.reset.email') }}
        <input v-model="email" type="email" required autocomplete="email"
          class="mt-1.5 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none transition focus:border-gold focus:ring-2 focus:ring-gold/20" />
      </label>

      <p v-if="err" class="mt-4 text-sm text-red-400" role="alert">{{ err }}</p>

      <button type="submit" :disabled="requestBusy"
        class="mt-7 w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep disabled:opacity-50">
        {{ requestBusy ? '…' : $t('authmail.reset.submit') }}
      </button>
      <p class="mt-6 text-center text-sm text-dim">
        <NuxtLink :to="localePath('/login')" class="text-gold font-semibold transition hover:text-gold-deep">{{ $t('authmail.reset.backToLogin') }}</NuxtLink>
      </p>
    </form>

    <!-- Request sent confirmation -->
    <div v-else-if="!token && requestSent" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-10 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center justify-center gap-2 text-lg font-extrabold tracking-tight mb-8"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
      <span class="status-icon status-icon--gold mx-auto" aria-hidden="true">✓</span>
      <h1 class="mt-5 text-2xl font-bold">{{ $t('authmail.reset.sent') }}</h1>
      <p class="mt-3 text-sm text-dim">{{ $t('authmail.reset.sentDesc') }}</p>
      <NuxtLink :to="localePath('/login')" class="mt-7 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep">
        {{ $t('authmail.reset.backToLogin') }}
      </NuxtLink>
    </div>

    <!-- Token present: set a new password -->
    <form v-else-if="token && confirmState === 'idle'" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-10" @submit.prevent="confirmReset">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center justify-center gap-2 text-lg font-extrabold tracking-tight mb-8"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold text-center">{{ $t('authmail.reset.newPasswordTitle') }}</h1>

      <label class="block mt-8 text-sm text-dim">{{ $t('authmail.reset.newPassword') }}
        <input v-model="password" type="password" required autocomplete="new-password" minlength="8"
          class="mt-1.5 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none transition focus:border-gold focus:ring-2 focus:ring-gold/20" />
      </label>

      <p v-if="err" class="mt-4 text-sm text-red-400" role="alert">{{ err }}</p>

      <button type="submit" :disabled="confirmBusy"
        class="mt-7 w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep disabled:opacity-50">
        {{ confirmBusy ? '…' : $t('authmail.reset.confirmSubmit') }}
      </button>
    </form>

    <!-- Reset success -->
    <div v-else-if="confirmState === 'success'" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-10 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center justify-center gap-2 text-lg font-extrabold tracking-tight mb-8"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
      <span class="status-icon status-icon--gold mx-auto" aria-hidden="true">✓</span>
      <h1 class="mt-5 text-2xl font-bold">{{ $t('authmail.reset.success') }}</h1>
      <p class="mt-3 text-sm text-dim">{{ $t('authmail.reset.successDesc') }}</p>
      <NuxtLink :to="localePath('/login')" class="mt-7 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep">
        {{ $t('authmail.reset.backToLogin') }}
      </NuxtLink>
    </div>

    <!-- Token invalid/expired -->
    <div v-else class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-10 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center justify-center gap-2 text-lg font-extrabold tracking-tight mb-8"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
      <span class="status-icon status-icon--danger mx-auto" aria-hidden="true">✕</span>
      <h1 class="mt-5 text-2xl font-bold text-red-400">{{ $t('authmail.reset.invalid') }}</h1>
      <p class="mt-3 text-sm text-dim">{{ $t('authmail.reset.invalidDesc') }}</p>
      <NuxtLink :to="localePath('/reset')" class="mt-7 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep">
        {{ $t('authmail.reset.submit') }}
      </NuxtLink>
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
