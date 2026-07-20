<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
const { supported, capabilities, conditionalMediationAvailable, login: passkeyLogin } = usePasskeys()

useSeoMeta({ title: () => `${t('auth.login.title')} — NOXIOAI` })

const identifier = ref('')
const password = ref('')
const remember = ref(false)
const err = ref('')
const busy = ref(false)
const passkeyBusy = ref(false)
const authCapabilities = ref<AuthCapabilities | null>(null)

const passkeyAvailable = computed(() => supported.value && authCapabilities.value?.passkeys === true)

let conditionalAbort: AbortController | null = null

onMounted(async () => {
  try {
    authCapabilities.value = await capabilities()
  } catch {
    authCapabilities.value = null
  }
  startConditionalPasskey()
})

onBeforeUnmount(() => {
  conditionalAbort?.abort()
  conditionalAbort = null
})

// Offers passkeys in the browser's username autofill without opening a modal;
// aborted silently when the user signs in another way or leaves the page.
async function startConditionalPasskey() {
  if (!passkeyAvailable.value || !await conditionalMediationAvailable()) return
  conditionalAbort = new AbortController()
  try {
    await passkeyLogin(remember.value, { mediation: 'conditional', signal: conditionalAbort.signal })
    await navigateTo(localePath('/app'))
  } catch {
    // Abort and dismissal are normal outcomes here; explicit flows report errors.
  }
}

async function submit() {
  conditionalAbort?.abort()
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
  conditionalAbort?.abort()
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
          autocomplete="username webauthn"
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
