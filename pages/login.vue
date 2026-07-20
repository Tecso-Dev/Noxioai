<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
const { supported, capabilities, login: passkeyLogin } = usePasskeys()

useSeoMeta({ title: () => `${t('auth.login.title')} — NOXIOAI` })

const identifier = ref('')
const password = ref('')
const remember = ref(false)
const err = ref('')
const busy = ref(false)
const passkeyBusy = ref(false)
const authCapabilities = ref<AuthCapabilities | null>(null)

const passkeyAvailable = computed(() => supported.value && authCapabilities.value?.passkeys === true)

onMounted(async () => {
  try {
    authCapabilities.value = await capabilities()
  } catch {
    authCapabilities.value = null
  }
})

async function submit() {
  err.value = ''
  busy.value = true
  try {
    const response = await $fetch<{ verified: boolean }>(`${api}/api/auth/login`, {
      method: 'POST', credentials: 'include',
      body: { identifier: identifier.value, password: password.value, remember: remember.value }
    })
    if (!response.verified) {
      err.value = t('auth.login.verifyEmail')
      return
    }
    await navigateTo(localePath('/app'))
  } catch {
    err.value = t('auth.login.invalidCredentials')
  } finally {
    busy.value = false
  }
}

async function usePasskey() {
  err.value = ''
  passkeyBusy.value = true
  try {
    await passkeyLogin(remember.value)
    await navigateTo(localePath('/app'))
  } catch {
    err.value = t('auth.login.passkeyError')
  } finally {
    passkeyBusy.value = false
  }
}
</script>

<template>
  <AuthShell
    :eyebrow="$t('auth.login.eyebrow')"
    :title="$t('auth.login.title')"
    :description="$t('auth.login.description')"
  >
    <form class="auth-form" novalidate @submit.prevent="submit">
      <div class="auth-field">
        <label for="login-identifier">{{ $t('auth.login.identifier') }}</label>
        <input
          id="login-identifier"
          v-model.trim="identifier"
          name="username"
          type="text"
          required
          autocomplete="username"
          autocapitalize="none"
          spellcheck="false"
          autofocus
          :aria-invalid="!!err || undefined"
        >
      </div>

      <AuthPasswordField
        id="login-password"
        v-model="password"
        :label="$t('auth.login.password')"
        autocomplete="current-password"
        :invalid="!!err"
      />

      <div class="auth-form__options">
        <label class="auth-check">
          <input v-model="remember" type="checkbox">
          <span><strong>{{ $t('auth.login.remember') }}</strong><small>{{ $t('auth.login.privateDevice') }}</small></span>
        </label>
        <NuxtLink :to="localePath('/reset')" class="auth-text-link">{{ $t('authmail.forgotPassword') }}</NuxtLink>
      </div>

      <p v-if="err" class="auth-alert" role="alert">{{ err }}</p>

      <button class="auth-primary" type="submit" :disabled="busy || passkeyBusy">
        <span>{{ busy ? $t('auth.working') : $t('auth.login.submit') }}</span>
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14M14 7l5 5-5 5"/></svg>
      </button>

      <div class="auth-divider"><span>{{ $t('auth.or') }}</span></div>
      <a class="auth-secondary" :href="`${api}/api/auth/google/start`">
        <svg viewBox="0 0 24 24" aria-hidden="true" style="fill:currentColor;stroke:none">
          <path d="M23.52 12.27c0-.82-.07-1.6-.2-2.36H12v4.47h6.47c-.28 1.5-1.13 2.78-2.4 3.63v3.02h3.88c2.27-2.09 3.57-5.17 3.57-8.76Z"/>
          <path d="M12 24c3.24 0 5.96-1.07 7.95-2.9l-3.88-3.02c-1.08.72-2.45 1.15-4.07 1.15-3.13 0-5.78-2.11-6.73-4.95H1.27v3.11C3.25 21.3 7.31 24 12 24Z"/>
          <path d="M5.27 14.28A7.2 7.2 0 0 1 4.9 12c0-.79.14-1.56.37-2.28V6.61H1.27A11.98 11.98 0 0 0 0 12c0 1.94.46 3.77 1.27 5.39l4-3.11Z"/>
          <path d="M12 4.77c1.77 0 3.35.61 4.6 1.8l3.44-3.44C17.95 1.19 15.24 0 12 0 7.31 0 3.25 2.7 1.27 6.61l4 3.11C6.22 6.88 8.87 4.77 12 4.77Z"/>
        </svg>
        {{ $t('auth.google') }}
      </a>

      <template v-if="passkeyAvailable">
        <div class="auth-divider"><span>{{ $t('auth.or') }}</span></div>
        <button class="auth-secondary" type="button" :disabled="busy || passkeyBusy" @click="usePasskey">
          <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="8" cy="15" r="3"/><path d="M10.5 13.3 20 4m-3 3 2 2m-5-5 2 2"/></svg>
          {{ passkeyBusy ? $t('auth.working') : $t('auth.login.passkey') }}
        </button>
      </template>

      <p class="auth-switch">
        {{ $t('auth.login.noAccount') }}
        <NuxtLink :to="localePath('/signup')">{{ $t('auth.login.signupLink') }}</NuxtLink>
      </p>
    </form>
  </AuthShell>
</template>
