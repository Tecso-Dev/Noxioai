<script setup lang="ts">
interface AccountUser {
  name?: string
  username?: string
  email?: string
  locale?: string
  verified?: boolean
}

interface AccountSession {
  id: string
  created_at: string
  last_seen_at: string
  user_agent: string
  ip_hint: string
  remembered: boolean
  auth_method: string
  expires_at: string
  current: boolean
}

interface AccountPasskey {
  id: number
  name: string
  created_at: string
  last_used_at?: string
}

const { t, locale } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
const { supported, capabilities: getCapabilities, register: registerCredential } = usePasskeys()

useSeoMeta({ title: () => `${t('auth.account.title')} — NOXIOAI` })

const me = ref<AccountUser | null>(null)
const sessions = ref<AccountSession[]>([])
const passkeys = ref<AccountPasskey[]>([])
const authCapabilities = ref<AuthCapabilities | null>(null)
const loading = ref(true)
const actionBusy = ref('')
const message = ref('')
const error = ref('')

const dateFormatter = computed(() => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }))
const passkeyReady = computed(() => me.value?.verified && supported.value && authCapabilities.value?.passkeys)

function formatDate(value: string) {
  try { return dateFormatter.value.format(new Date(value)) } catch { return value }
}

function deviceName(userAgent: string) {
  if (!userAgent) return t('auth.account.unknownDevice')
  if (/iPhone/i.test(userAgent)) return 'iPhone'
  if (/Android/i.test(userAgent)) return 'Android'
  if (/Macintosh|Mac OS/i.test(userAgent)) return 'Mac'
  if (/Windows/i.test(userAgent)) return 'Windows PC'
  if (/Linux/i.test(userAgent)) return 'Linux'
  return t('auth.account.browserSession')
}

async function loadSecurity() {
  loading.value = true
  try {
    me.value = await $fetch<AccountUser>(`${api}/api/auth/me`, { credentials: 'include' })
  } catch {
    await navigateTo(localePath('/login'))
    return
  }
  const [sessionResult, passkeyResult, capabilityResult] = await Promise.allSettled([
    $fetch<{ sessions: AccountSession[] }>(`${api}/api/auth/sessions`, { credentials: 'include' }),
    $fetch<{ passkeys: AccountPasskey[] }>(`${api}/api/auth/passkeys`, { credentials: 'include' }),
    getCapabilities()
  ])
  if (sessionResult.status === 'fulfilled') sessions.value = sessionResult.value.sessions
  if (passkeyResult.status === 'fulfilled') passkeys.value = passkeyResult.value.passkeys
  if (capabilityResult.status === 'fulfilled') authCapabilities.value = capabilityResult.value
  loading.value = false
}

onMounted(loadSecurity)

async function addPasskey() {
  message.value = ''
  error.value = ''
  actionBusy.value = 'passkey'
  try {
    await registerCredential()
    message.value = t('auth.account.passkeyAdded')
    const result = await $fetch<{ passkeys: AccountPasskey[] }>(`${api}/api/auth/passkeys`, { credentials: 'include' })
    passkeys.value = result.passkeys
  } catch {
    error.value = t('auth.account.passkeyError')
  } finally {
    actionBusy.value = ''
  }
}

async function deletePasskey(id: number) {
  message.value = ''
  error.value = ''
  actionBusy.value = `passkey-${id}`
  try {
    await $fetch(`${api}/api/auth/passkeys/${id}`, { method: 'DELETE', credentials: 'include' })
    passkeys.value = passkeys.value.filter(item => item.id !== id)
    message.value = t('auth.account.passkeyRemoved')
  } catch {
    error.value = t('auth.error')
  } finally {
    actionBusy.value = ''
  }
}

async function endSession(session: AccountSession) {
  actionBusy.value = `session-${session.id}`
  error.value = ''
  try {
    await $fetch(`${api}/api/auth/sessions/${session.id}`, { method: 'DELETE', credentials: 'include' })
    if (session.current) {
      await navigateTo(localePath('/login'))
      return
    }
    sessions.value = sessions.value.filter(item => item.id !== session.id)
    message.value = t('auth.account.sessionEnded')
  } catch {
    error.value = t('auth.error')
  } finally {
    actionBusy.value = ''
  }
}

async function logoutAll() {
  actionBusy.value = 'all-sessions'
  try {
    await $fetch(`${api}/api/auth/logout-all`, { method: 'POST', credentials: 'include' })
  } finally {
    await navigateTo(localePath('/login'))
  }
}

async function logout() {
  try { await $fetch(`${api}/api/auth/logout`, { method: 'POST', credentials: 'include' }) } catch {}
  await navigateTo(localePath('/login'))
}
</script>

<template>
  <main class="security-page">
    <div class="site-grid" aria-hidden="true" />
    <header class="security-header">
      <NuxtLink :to="localePath('/app')" class="auth-brand">
        <img src="/brand/mark-dark.png" alt="" width="32" height="32">
        <span>NOXIO<span>AI</span></span>
      </NuxtLink>
      <button class="security-logout" type="button" @click="logout">{{ $t('auth.account.logout') }}</button>
    </header>

    <div class="security-content">
      <div class="security-heading">
        <div>
          <p>{{ $t('auth.account.eyebrow') }}</p>
          <h1>{{ $t('auth.account.title') }}</h1>
          <div>{{ $t('auth.account.description') }}</div>
        </div>
        <span class="security-score"><i aria-hidden="true" />{{ $t('auth.account.protected') }}</span>
      </div>

      <div v-if="loading" class="security-loading" role="status">{{ $t('auth.working') }}</div>

      <template v-else>
        <p v-if="message" class="security-message" role="status">{{ message }}</p>
        <p v-if="error" class="auth-alert" role="alert">{{ error }}</p>

        <section class="security-card" aria-labelledby="profile-title">
          <div class="security-card__head">
            <div><span>01</span><h2 id="profile-title">{{ $t('auth.account.profile') }}</h2></div>
            <span class="security-badge" :class="{ warning: !me?.verified }">{{ me?.verified ? $t('auth.account.verified') : $t('auth.account.unverified') }}</span>
          </div>
          <dl class="security-profile-grid">
            <div><dt>{{ $t('auth.account.name') }}</dt><dd>{{ me?.name || '—' }}</dd></div>
            <div><dt>{{ $t('auth.account.username') }}</dt><dd>{{ me?.username ? `@${me.username}` : '—' }}</dd></div>
            <div><dt>{{ $t('auth.account.email') }}</dt><dd>{{ me?.email || '—' }}</dd></div>
            <div><dt>{{ $t('auth.account.locale') }}</dt><dd>{{ me?.locale?.toUpperCase() || '—' }}</dd></div>
          </dl>
          <NuxtLink :to="localePath('/onboarding')" class="security-link">{{ $t('onboarding.editProfile') }} →</NuxtLink>
        </section>

        <section class="security-card" aria-labelledby="passkeys-title">
          <div class="security-card__head">
            <div><span>02</span><h2 id="passkeys-title">{{ $t('auth.account.passkeys') }}</h2></div>
            <span class="security-count">{{ passkeys.length }}</span>
          </div>
          <p class="security-card__description">{{ $t('auth.account.passkeysDescription') }}</p>

          <ul v-if="passkeys.length" class="security-list">
            <li v-for="passkey in passkeys" :key="passkey.id">
              <span class="security-item-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><circle cx="8" cy="15" r="3"/><path d="M10.5 13.3 20 4m-3 3 2 2m-5-5 2 2"/></svg></span>
              <span class="security-item-copy"><strong>{{ passkey.name }}</strong><small>{{ $t('auth.account.added') }} {{ formatDate(passkey.created_at) }}</small></span>
              <button type="button" :disabled="actionBusy === `passkey-${passkey.id}`" @click="deletePasskey(passkey.id)">{{ $t('auth.account.remove') }}</button>
            </li>
          </ul>

          <button v-if="passkeyReady" class="security-action" type="button" :disabled="actionBusy === 'passkey'" @click="addPasskey">
            + {{ actionBusy === 'passkey' ? $t('auth.working') : $t('auth.account.addPasskey') }}
          </button>
          <p v-else class="security-unavailable">{{ authCapabilities?.passkeys ? $t('auth.account.passkeyBrowserUnavailable') : $t('auth.account.passkeyServerUnavailable') }}</p>
        </section>

        <section class="security-card" aria-labelledby="sessions-title">
          <div class="security-card__head">
            <div><span>03</span><h2 id="sessions-title">{{ $t('auth.account.sessions') }}</h2></div>
            <span class="security-count">{{ sessions.length }}</span>
          </div>
          <p class="security-card__description">{{ $t('auth.account.sessionsDescription') }}</p>
          <ul class="security-list">
            <li v-for="session in sessions" :key="session.id">
              <span class="security-item-icon" aria-hidden="true"><svg viewBox="0 0 24 24"><rect x="3" y="5" width="18" height="13" rx="2"/><path d="M8 21h8M12 18v3"/></svg></span>
              <span class="security-item-copy">
                <strong>{{ deviceName(session.user_agent) }} <em v-if="session.current">{{ $t('auth.account.current') }}</em></strong>
                <small>{{ formatDate(session.last_seen_at) }} · {{ session.ip_hint || $t('auth.account.networkHidden') }} · {{ session.auth_method }}</small>
              </span>
              <button type="button" :disabled="actionBusy === `session-${session.id}`" @click="endSession(session)">{{ session.current ? $t('auth.account.logout') : $t('auth.account.end') }}</button>
            </li>
          </ul>
          <button class="security-action security-action--danger" type="button" :disabled="actionBusy === 'all-sessions'" @click="logoutAll">{{ $t('auth.account.endAll') }}</button>
        </section>
      </template>
    </div>
  </main>
</template>

<style scoped>
.security-page { background: #040914; color: var(--ivory); min-block-size: 100dvh; padding: 1.5rem clamp(1.25rem, 5vw, 5rem) 5rem; position: relative; }
.security-header { align-items: center; display: flex; justify-content: space-between; margin: 0 auto; max-inline-size: 68rem; position: relative; z-index: 2; }
.auth-brand { align-items: center; color: var(--ivory); display: inline-flex; font-size: 1rem; font-weight: 800; gap: .65rem; letter-spacing: -.04em; text-decoration: none; }
.auth-brand img { block-size: 2rem; border: 1px solid rgba(212,191,148,.28); border-radius: 50%; inline-size: 2rem; }
.auth-brand span span { color: var(--gold); }
.security-logout { background: transparent; border: 1px solid rgba(151,179,207,.2); color: #9eacbc; cursor: pointer; font-size: .7rem; padding: .6rem .9rem; }
.security-content { margin: 4.5rem auto 0; max-inline-size: 58rem; position: relative; z-index: 1; }
.security-heading { align-items: flex-end; display: flex; gap: 2rem; justify-content: space-between; margin-block-end: 2rem; }
.security-heading p { color: var(--cyan); font-family: 'DM Mono', monospace; font-size: .62rem; letter-spacing: .14em; margin: 0 0 .75rem; text-transform: uppercase; }
.security-heading h1 { font-size: clamp(2rem, 5vw, 3.25rem); letter-spacing: -.055em; line-height: 1; margin: 0; }
.security-heading > div > div { color: #8191a4; font-size: .8rem; margin-block-start: .8rem; }
.security-score { align-items: center; border: 1px solid rgba(89,212,170,.22); color: #87cdb5; display: flex; font-family: 'DM Mono', monospace; font-size: .6rem; gap: .5rem; letter-spacing: .08em; padding: .55rem .75rem; text-transform: uppercase; }
.security-score i { background: #59d4aa; border-radius: 50%; box-shadow: 0 0 12px rgba(89,212,170,.5); block-size: .4rem; inline-size: .4rem; }
.security-card { background: rgba(5,17,31,.78); border: 1px solid rgba(151,179,207,.14); margin-block-start: 1rem; padding: clamp(1.25rem, 4vw, 2rem); }
.security-card__head { align-items: center; display: flex; justify-content: space-between; }
.security-card__head > div { align-items: center; display: flex; gap: .75rem; }
.security-card__head > div > span { color: #718399; font-family: 'DM Mono', monospace; font-size: .58rem; }
.security-card h2 { font-size: 1rem; letter-spacing: -.02em; margin: 0; }
.security-badge, .security-count { border: 1px solid rgba(89,212,170,.2); color: #75c6aa; font-family: 'DM Mono', monospace; font-size: .57rem; padding: .35rem .5rem; text-transform: uppercase; }
.security-badge.warning { border-color: rgba(212,191,148,.24); color: var(--gold); }
.security-count { border-color: rgba(72,202,228,.18); color: var(--cyan); min-inline-size: 1.7rem; text-align: center; }
.security-card__description { color: #77889c; font-size: .73rem; line-height: 1.65; margin: .8rem 0 1.25rem; max-inline-size: 65ch; }
.security-profile-grid { display: grid; gap: 1rem; grid-template-columns: repeat(2, 1fr); margin: 1.5rem 0; }
.security-profile-grid div { border-inline-start: 1px solid rgba(72,202,228,.2); padding-inline-start: .75rem; }
.security-profile-grid dt { color: #718399; font-size: .58rem; letter-spacing: .08em; text-transform: uppercase; }
.security-profile-grid dd { color: #d5dee7; font-size: .8rem; margin: .35rem 0 0; overflow-wrap: anywhere; }
.security-link { color: var(--gold); font-size: .68rem; text-decoration: none; }
.security-list { display: grid; list-style: none; margin: 0 0 1rem; padding: 0; }
.security-list li { align-items: center; border-block-start: 1px solid rgba(151,179,207,.1); display: flex; gap: .85rem; min-inline-size: 0; padding: .9rem 0; }
.security-item-icon { align-items: center; background: rgba(72,202,228,.06); border: 1px solid rgba(72,202,228,.15); color: var(--cyan); display: flex; flex: 0 0 auto; justify-content: center; block-size: 2.25rem; inline-size: 2.25rem; }
.security-item-icon svg { fill: none; stroke: currentColor; stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.4; block-size: 1.1rem; inline-size: 1.1rem; }
.security-item-copy { flex: 1; min-inline-size: 0; }
.security-item-copy strong, .security-item-copy small { display: block; }
.security-item-copy strong { color: #c8d3de; font-size: .75rem; }
.security-item-copy strong em { background: rgba(89,212,170,.08); color: #75c6aa; font-size: .52rem; font-style: normal; margin-inline-start: .4rem; padding: .2rem .35rem; text-transform: uppercase; }
.security-item-copy small { color: #718399; font-size: .62rem; margin-block-start: .25rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.security-list button { background: transparent; border: 0; color: #8697aa; cursor: pointer; flex: 0 0 auto; font-size: .62rem; padding: .5rem; }
.security-list button:hover { color: #fca5a5; }
.security-action { background: rgba(72,202,228,.06); border: 1px solid rgba(72,202,228,.22); color: #a8dce8; cursor: pointer; font-size: .67rem; padding: .65rem .85rem; }
.security-action--danger { background: rgba(248,113,113,.04); border-color: rgba(248,113,113,.18); color: #e7a0a0; }
.security-unavailable { color: #718399; font-size: .65rem; margin: 0; }
.security-message { background: rgba(89,212,170,.06); border: 1px solid rgba(89,212,170,.2); color: #8dd0ba; font-size: .7rem; padding: .7rem .8rem; }
.security-loading { color: #718399; font-family: 'DM Mono', monospace; font-size: .7rem; padding: 4rem; text-align: center; }
@media (max-width: 620px) { .security-content { margin-block-start: 3rem; } .security-heading { align-items: flex-start; flex-direction: column; } .security-profile-grid { grid-template-columns: 1fr; } .security-list li { align-items: flex-start; flex-wrap: wrap; } .security-list button { margin-inline-start: 3.1rem; } }
</style>
