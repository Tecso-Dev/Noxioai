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
    <form class="glass-card auth-card relative z-10 w-full max-w-sm rounded-2xl p-10" @submit.prevent="submit">
      <NuxtLink :to="localePath('/')" class="brand-mark flex items-center justify-center gap-2 text-lg font-extrabold tracking-tight mb-8"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
      <h1 class="text-2xl font-bold text-center">{{ $t('auth.login.title') }}</h1>

      <label class="block mt-8 text-sm text-dim">{{ $t('auth.login.email') }}
        <input v-model="email" type="email" required autocomplete="email"
          class="mt-1.5 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none transition focus:border-gold focus:ring-2 focus:ring-gold/20" />
      </label>
      <label class="block mt-4 text-sm text-dim">{{ $t('auth.login.password') }}
        <input v-model="password" type="password" required autocomplete="current-password" minlength="8"
          class="mt-1.5 w-full rounded-xl border border-line bg-panel px-4 py-3 outline-none transition focus:border-gold focus:ring-2 focus:ring-gold/20" />
      </label>
      <p class="mt-3 text-end">
        <NuxtLink :to="localePath('/reset')" class="text-sm text-dim font-semibold transition hover:text-gold">{{ $t('authmail.forgotPassword') }}</NuxtLink>
      </p>

      <p v-if="err" class="mt-4 text-sm text-red-400" role="alert">{{ err }}</p>

      <button type="submit" :disabled="busy"
        class="mt-7 w-full rounded-full bg-brand px-5 py-3 font-bold text-night transition hover:bg-gold-deep disabled:opacity-50">
        {{ busy ? '…' : $t('auth.login.submit') }}
      </button>
      <p class="mt-6 text-center text-sm text-dim">
        {{ $t('auth.login.noAccount') }}
        <NuxtLink :to="localePath('/signup')" class="text-gold font-semibold transition hover:text-gold-deep">{{ $t('auth.login.signupLink') }}</NuxtLink>
      </p>
    </form>
  </main>
</template>
