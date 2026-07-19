<script setup lang="ts">
const { t, locale } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase

useSeoMeta({ title: () => `${t('auth.signup.title')} — NOXIOAI` })

const name = ref('')
const username = ref('')
const email = ref('')
const password = ref('')
const confirmation = ref('')
const accepted = ref(false)
const err = ref('')
const busy = ref(false)
const complete = ref(false)

const passwordLength = computed(() => Array.from(password.value).length)
const longEnough = computed(() => passwordLength.value >= 15)
const matches = computed(() => confirmation.value.length > 0 && confirmation.value === password.value)
const strength = computed(() => {
  let score = 0
  if (longEnough.value) score += 2
  if (passwordLength.value >= 20) score += 1
  if (/\s/u.test(password.value)) score += 1
  if (new Set(Array.from(password.value)).size >= 10) score += 1
  return Math.min(score, 4)
})

function localValidation(): string {
  if (!/^[a-zA-Z0-9][a-zA-Z0-9._-]{2,31}$/.test(username.value.trim())) return t('auth.signup.usernameError')
  if (!longEnough.value) return t('auth.signup.passwordLength')
  if (!matches.value) return t('auth.signup.passwordMismatch')
  if (!accepted.value) return t('auth.signup.acceptRequired')
  return ''
}

async function submit() {
  err.value = localValidation()
  if (err.value) return
  busy.value = true
  try {
    await $fetch(`${api}/api/auth/signup`, {
      method: 'POST', credentials: 'include',
      body: {
        name: name.value,
        username: username.value,
        email: email.value,
        password: password.value,
        locale: locale.value,
        accept_terms: accepted.value
      }
    })
    complete.value = true
    password.value = ''
    confirmation.value = ''
  } catch (error: any) {
    const code = error?.data?.error
    if (code === 'password_compromised') err.value = t('auth.signup.passwordCompromised')
    else if (code === 'password_screening_unavailable') err.value = t('auth.signup.passwordScreeningUnavailable')
    else if (code === 'password_too_short') err.value = t('auth.signup.passwordLength')
    else if (code === 'invalid_username') err.value = t('auth.signup.usernameError')
    else err.value = t('auth.error')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <AuthShell
    :eyebrow="$t('auth.signup.eyebrow')"
    :title="complete ? $t('auth.signup.checkEmailTitle') : $t('auth.signup.title')"
    :description="complete ? $t('auth.signup.checkEmailDescription') : $t('auth.signup.description')"
    wide
  >
    <div v-if="complete" class="auth-complete" role="status">
      <span class="auth-complete__icon" aria-hidden="true">
        <svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></svg>
      </span>
      <p>{{ $t('auth.signup.checkEmailPrivacy') }}</p>
      <NuxtLink :to="localePath('/login')" class="auth-primary">{{ $t('auth.signup.loginLink') }}</NuxtLink>
    </div>

    <form v-else class="auth-form auth-form--signup" novalidate @submit.prevent="submit">
      <div class="auth-field-row">
        <div class="auth-field">
          <label for="signup-name">{{ $t('auth.signup.name') }}</label>
          <input id="signup-name" v-model.trim="name" name="name" type="text" required autocomplete="name" maxlength="100">
        </div>
        <div class="auth-field">
          <label for="signup-username">{{ $t('auth.signup.username') }}</label>
          <input id="signup-username" v-model.trim="username" name="username" type="text" required autocomplete="username" autocapitalize="none" spellcheck="false" minlength="3" maxlength="32" aria-describedby="username-hint">
          <small id="username-hint" class="auth-hint">{{ $t('auth.signup.usernameHint') }}</small>
        </div>
      </div>

      <div class="auth-field">
        <label for="signup-email">{{ $t('auth.signup.email') }}</label>
        <input id="signup-email" v-model.trim="email" name="email" type="email" required autocomplete="email" autocapitalize="none" spellcheck="false">
      </div>

      <AuthPasswordField
        id="signup-password"
        v-model="password"
        :label="$t('auth.signup.password')"
        autocomplete="new-password"
        describedby="password-guidance"
        :invalid="!!err && !longEnough"
      />

      <AuthPasswordField
        id="signup-password-confirmation"
        v-model="confirmation"
        :label="$t('auth.signup.confirmPassword')"
        autocomplete="new-password"
        :invalid="confirmation.length > 0 && !matches"
      />

      <div id="password-guidance" class="auth-password-guide">
        <div class="auth-strength" aria-hidden="true">
          <span v-for="index in 4" :key="index" :class="{ active: strength >= index }" />
        </div>
        <p>{{ $t('auth.signup.passwordGuidance') }}</p>
        <ul>
          <li :class="{ met: longEnough }">{{ $t('auth.signup.passwordLength') }}</li>
          <li :class="{ met: password.includes(' ') }">{{ $t('auth.signup.passphraseTip') }}</li>
          <li :class="{ met: matches }">{{ $t('auth.signup.passwordsMatch') }}</li>
        </ul>
      </div>

      <label class="auth-check auth-check--terms">
        <input v-model="accepted" type="checkbox" required>
        <span>
          <i18n-t keypath="auth.signup.acceptTerms" tag="small">
            <template #terms><NuxtLink :to="localePath('/terms')" target="_blank">{{ $t('auth.signup.terms') }}</NuxtLink></template>
            <template #privacy><NuxtLink :to="localePath('/privacy')" target="_blank">{{ $t('auth.signup.privacy') }}</NuxtLink></template>
          </i18n-t>
        </span>
      </label>

      <p v-if="err" class="auth-alert" role="alert">{{ err }}</p>

      <button class="auth-primary" type="submit" :disabled="busy">
        <span>{{ busy ? $t('auth.working') : $t('auth.signup.submit') }}</span>
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14M14 7l5 5-5 5"/></svg>
      </button>

      <p class="auth-passkey-note">
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="8" cy="15" r="3"/><path d="M10.5 13.3 20 4m-3 3 2 2m-5-5 2 2"/></svg>
        {{ $t('auth.signup.passkeyAfterVerify') }}
      </p>

      <p class="auth-switch">
        {{ $t('auth.signup.haveAccount') }}
        <NuxtLink :to="localePath('/login')">{{ $t('auth.signup.loginLink') }}</NuxtLink>
      </p>
    </form>
  </AuthShell>
</template>
