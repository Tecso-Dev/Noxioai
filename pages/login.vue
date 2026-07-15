<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('auth.login.title')} — NOXIOAI` })

const email = ref('')
const password = ref('')
const err = ref('')
const busy = ref(false)

async function submit() {
  err.value = ''
  busy.value = true
  try {
    await $fetch(`${api}/api/auth/login`, {
      method: 'POST', credentials: 'include',
      body: { email: email.value, password: password.value }
    })
    await navigateTo(localePath('/app'))
  } catch (e: any) {
    err.value = e?.data || t('auth.error')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="auth-wrap min-h-dvh flex items-center justify-center px-6 py-16">
    <div class="site-grid" aria-hidden="true" />
    <form class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-8" @submit.prevent="submit">
      <NuxtLink :to="localePath('/')" class="brand-mark block text-center text-lg font-extrabold tracking-tight mb-6">NOXIO<span class="text-brand2">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold text-center">{{ $t('auth.login.title') }}</h1>

      <label class="block mt-6 text-sm text-dim">{{ $t('auth.login.email') }}
        <input v-model="email" type="email" required autocomplete="email"
          class="mt-1 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none focus:border-brand2 transition" />
      </label>
      <label class="block mt-4 text-sm text-dim">{{ $t('auth.login.password') }}
        <input v-model="password" type="password" required autocomplete="current-password" minlength="8"
          class="mt-1 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none focus:border-brand2 transition" />
      </label>
      <p class="mt-2 text-end">
        <NuxtLink :to="localePath('/reset')" class="text-sm text-brand2 font-semibold">{{ $t('authmail.forgotPassword') }}</NuxtLink>
      </p>

      <p v-if="err" class="mt-4 text-sm text-red" role="alert">{{ err }}</p>

      <button type="submit" :disabled="busy"
        class="mt-6 w-full rounded-full bg-brand px-5 py-3 font-bold text-night disabled:opacity-50 transition">
        {{ busy ? '…' : $t('auth.login.submit') }}
      </button>
      <p class="mt-5 text-center text-sm text-dim">
        {{ $t('auth.login.noAccount') }}
        <NuxtLink :to="localePath('/signup')" class="text-brand2 font-semibold">{{ $t('auth.login.signupLink') }}</NuxtLink>
      </p>
    </form>
  </main>
</template>
