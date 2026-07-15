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
    <form v-if="!token && !requestSent" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-8" @submit.prevent="requestLink">
      <NuxtLink :to="localePath('/')" class="brand-mark block text-center text-lg font-extrabold tracking-tight mb-6">NOXIO<span class="text-brand2">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold text-center">{{ $t('authmail.reset.title') }}</h1>
      <p class="mt-2 text-center text-sm text-dim">{{ $t('authmail.reset.desc') }}</p>

      <label class="block mt-6 text-sm text-dim">{{ $t('authmail.reset.email') }}
        <input v-model="email" type="email" required autocomplete="email"
          class="mt-1 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none focus:border-brand2 transition" />
      </label>

      <p v-if="err" class="mt-4 text-sm text-red" role="alert">{{ err }}</p>

      <button type="submit" :disabled="requestBusy"
        class="mt-6 w-full rounded-full bg-brand px-5 py-3 font-bold text-night disabled:opacity-50 transition">
        {{ requestBusy ? '…' : $t('authmail.reset.submit') }}
      </button>
      <p class="mt-5 text-center text-sm text-dim">
        <NuxtLink :to="localePath('/login')" class="text-brand2 font-semibold">{{ $t('authmail.reset.backToLogin') }}</NuxtLink>
      </p>
    </form>

    <!-- Request sent confirmation -->
    <div v-else-if="!token && requestSent" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-8 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark block text-lg font-extrabold tracking-tight mb-6">NOXIO<span class="text-brand2">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold">{{ $t('authmail.reset.sent') }}</h1>
      <p class="mt-3 text-sm text-dim">{{ $t('authmail.reset.sentDesc') }}</p>
      <NuxtLink :to="localePath('/login')" class="mt-6 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition">
        {{ $t('authmail.reset.backToLogin') }}
      </NuxtLink>
    </div>

    <!-- Token present: set a new password -->
    <form v-else-if="token && confirmState === 'idle'" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-8" @submit.prevent="confirmReset">
      <NuxtLink :to="localePath('/')" class="brand-mark block text-center text-lg font-extrabold tracking-tight mb-6">NOXIO<span class="text-brand2">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold text-center">{{ $t('authmail.reset.newPasswordTitle') }}</h1>

      <label class="block mt-6 text-sm text-dim">{{ $t('authmail.reset.newPassword') }}
        <input v-model="password" type="password" required autocomplete="new-password" minlength="8"
          class="mt-1 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none focus:border-brand2 transition" />
      </label>

      <p v-if="err" class="mt-4 text-sm text-red" role="alert">{{ err }}</p>

      <button type="submit" :disabled="confirmBusy"
        class="mt-6 w-full rounded-full bg-brand px-5 py-3 font-bold text-night disabled:opacity-50 transition">
        {{ confirmBusy ? '…' : $t('authmail.reset.confirmSubmit') }}
      </button>
    </form>

    <!-- Reset success -->
    <div v-else-if="confirmState === 'success'" class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-8 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark block text-lg font-extrabold tracking-tight mb-6">NOXIO<span class="text-brand2">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold">{{ $t('authmail.reset.success') }}</h1>
      <p class="mt-3 text-sm text-dim">{{ $t('authmail.reset.successDesc') }}</p>
      <NuxtLink :to="localePath('/login')" class="mt-6 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition">
        {{ $t('authmail.reset.backToLogin') }}
      </NuxtLink>
    </div>

    <!-- Token invalid/expired -->
    <div v-else class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-8 text-center">
      <NuxtLink :to="localePath('/')" class="brand-mark block text-lg font-extrabold tracking-tight mb-6">NOXIO<span class="text-brand2">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold">{{ $t('authmail.reset.invalid') }}</h1>
      <p class="mt-3 text-sm text-dim">{{ $t('authmail.reset.invalidDesc') }}</p>
      <NuxtLink :to="localePath('/reset')" class="mt-6 inline-block w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition">
        {{ $t('authmail.reset.submit') }}
      </NuxtLink>
    </div>
  </main>
</template>
