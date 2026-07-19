<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const route = useRoute()
const api = useRuntimeConfig().public.apiBase

useSeoMeta({ title: () => `${t('authmail.reset.title')} — NOXIOAI` })

const token = computed(() => typeof route.query.token === 'string' ? route.query.token : '')
const email = ref('')
const requestSent = ref(false)
const requestBusy = ref(false)
const password = ref('')
const confirmation = ref('')
const confirmState = ref<'idle' | 'success' | 'invalid'>('idle')
const confirmBusy = ref(false)
const err = ref('')

const shellTitle = computed(() => {
  if (!token.value) return requestSent.value ? t('authmail.reset.sent') : t('authmail.reset.title')
  if (confirmState.value === 'success') return t('authmail.reset.success')
  if (confirmState.value === 'invalid') return t('authmail.reset.invalid')
  return t('authmail.reset.newPasswordTitle')
})

const shellDescription = computed(() => {
  if (!token.value) return requestSent.value ? t('authmail.reset.sentDesc') : t('authmail.reset.desc')
  if (confirmState.value === 'success') return t('authmail.reset.successDesc')
  if (confirmState.value === 'invalid') return t('authmail.reset.invalidDesc')
  return t('authmail.reset.newPasswordDesc')
})

async function requestLink() {
  err.value = ''
  requestBusy.value = true
  try {
    await $fetch(`${api}/api/auth/reset/request`, {
      method: 'POST', credentials: 'include', body: { email: email.value }
    })
    requestSent.value = true
  } catch {
    err.value = t('auth.error')
  } finally {
    requestBusy.value = false
  }
}

async function confirmReset() {
  err.value = ''
  if (Array.from(password.value).length < 15) {
    err.value = t('auth.signup.passwordLength')
    return
  }
  if (password.value !== confirmation.value) {
    err.value = t('auth.signup.passwordMismatch')
    return
  }
  confirmBusy.value = true
  try {
    await $fetch(`${api}/api/auth/reset/confirm`, {
      method: 'POST', credentials: 'include', body: { token: token.value, password: password.value }
    })
    password.value = ''
    confirmation.value = ''
    confirmState.value = 'success'
  } catch (error: any) {
    if (error?.data?.error === 'password_compromised') err.value = t('auth.signup.passwordCompromised')
    else if (error?.data?.error === 'password_screening_unavailable') err.value = t('auth.signup.passwordScreeningUnavailable')
    else if (error?.status === 400) confirmState.value = 'invalid'
    else err.value = t('auth.error')
  } finally {
    confirmBusy.value = false
  }
}
</script>

<template>
  <AuthShell :eyebrow="$t('authmail.reset.eyebrow')" :title="shellTitle" :description="shellDescription">
    <form v-if="!token && !requestSent" class="auth-form" @submit.prevent="requestLink">
      <div class="auth-field">
        <label for="reset-email">{{ $t('authmail.reset.email') }}</label>
        <input id="reset-email" v-model.trim="email" name="email" type="email" required autocomplete="email" autocapitalize="none" spellcheck="false" autofocus>
      </div>
      <p v-if="err" class="auth-alert" role="alert">{{ err }}</p>
      <button class="auth-primary" type="submit" :disabled="requestBusy">{{ requestBusy ? $t('auth.working') : $t('authmail.reset.submit') }}</button>
      <p class="auth-switch"><NuxtLink :to="localePath('/login')">{{ $t('authmail.reset.backToLogin') }}</NuxtLink></p>
    </form>

    <div v-else-if="!token && requestSent" class="auth-complete" role="status">
      <span class="auth-complete__icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></svg></span>
      <p>{{ $t('authmail.reset.sentPrivacy') }}</p>
      <NuxtLink :to="localePath('/login')" class="auth-primary">{{ $t('authmail.reset.backToLogin') }}</NuxtLink>
    </div>

    <form v-else-if="confirmState === 'idle'" class="auth-form" @submit.prevent="confirmReset">
      <AuthPasswordField id="reset-password" v-model="password" :label="$t('authmail.reset.newPassword')" autocomplete="new-password" describedby="reset-password-hint" />
      <AuthPasswordField id="reset-confirmation" v-model="confirmation" :label="$t('auth.signup.confirmPassword')" autocomplete="new-password" :invalid="confirmation.length > 0 && password !== confirmation" />
      <small id="reset-password-hint" class="auth-hint">{{ $t('auth.signup.passwordGuidance') }}</small>
      <p v-if="err" class="auth-alert" role="alert">{{ err }}</p>
      <button class="auth-primary" type="submit" :disabled="confirmBusy">{{ confirmBusy ? $t('auth.working') : $t('authmail.reset.confirmSubmit') }}</button>
    </form>

    <div v-else class="auth-complete" role="status">
      <span class="auth-complete__icon" aria-hidden="true"><svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6"/></svg></span>
      <NuxtLink v-if="confirmState === 'success'" :to="localePath('/login')" class="auth-primary">{{ $t('authmail.reset.backToLogin') }}</NuxtLink>
      <NuxtLink v-else :to="localePath('/reset')" class="auth-primary">{{ $t('authmail.reset.submit') }}</NuxtLink>
    </div>
  </AuthShell>
</template>
