<script setup lang="ts">
type TenantBotStatus = {
  ok: boolean
  bot_username: string
  active: boolean
}

type TenantMessage = {
  id: number
  from_chat: string
  from_name: string
  customer_text: string
  agent_reply: string
  escalated: boolean
  created_at: string
}

type ConciergeChatMessage = {
  role: 'user' | 'assistant'
  content: string
}

const { t, locale } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('auth.app.greeting')} — NOXIOAI` })

const me = ref<{ name?: string; email?: string; verified: boolean } | null>(null)
const loading = ref(true)
const hasProfile = ref<boolean | null>(null)
const botStatus = ref<TenantBotStatus | null>(null)
const botToken = ref('')
const botLoading = ref(false)
const botBusy = ref(false)
const botError = ref('')
const botNotice = ref('')
const messages = ref<TenantMessage[]>([])
const messagesLoading = ref(false)
const messagesError = ref('')
const conciergeOpen = ref(false)
const conciergeMessages = ref<ConciergeChatMessage[]>([])
const conciergeInput = ref('')
const conciergeBusy = ref(false)
const conciergeError = ref('')
const conciergeMessageList = ref<HTMLElement | null>(null)

onMounted(async () => {
  try {
    me.value = await $fetch(`${api}/api/auth/me`, { credentials: 'include' })
  } catch {
    await navigateTo(localePath('/login'))
    return
  }

  if (me.value?.verified) {
    await Promise.all([
      (async () => {
        try {
          const profile = await $fetch<Record<string, unknown>>(`${api}/api/profile`, { credentials: 'include' })
          hasProfile.value = Object.keys(profile).length > 0
        } catch {
          hasProfile.value = null
        }
      })(),
      loadTenantBot(),
      loadTenantMessages()
    ])
  }
  loading.value = false
})

async function loadTenantBot() {
  botLoading.value = true
  try {
    botStatus.value = await $fetch<TenantBotStatus>(`${api}/api/bot`, { credentials: 'include' })
  } catch {
    botError.value = t('tenantbot.loadError')
  } finally {
    botLoading.value = false
  }
}

async function connectTenantBot() {
  botError.value = ''
  botNotice.value = ''
  if (!botToken.value.trim()) {
    botError.value = t('tenantbot.tokenRequired')
    return
  }

  botBusy.value = true
  try {
    const connected = await $fetch<{ ok: boolean; bot_username: string }>(`${api}/api/bot/connect`, {
      method: 'POST',
      credentials: 'include',
      body: { token: botToken.value.trim() }
    })
    botStatus.value = { ...connected, active: true }
    botToken.value = ''
    botNotice.value = t('tenantbot.connectSuccess')
  } catch {
    botError.value = t('tenantbot.connectError')
  } finally {
    botBusy.value = false
  }
}

async function disconnectTenantBot() {
  if (!window.confirm(t('tenantbot.disconnectConfirm'))) return

  botError.value = ''
  botNotice.value = ''
  botBusy.value = true
  try {
    await $fetch(`${api}/api/bot`, {
      method: 'DELETE',
      credentials: 'include'
    })
    botStatus.value = { ok: true, bot_username: '', active: false }
    botNotice.value = t('tenantbot.disconnectSuccess')
  } catch {
    botError.value = t('tenantbot.disconnectError')
  } finally {
    botBusy.value = false
  }
}

async function loadTenantMessages() {
  messagesLoading.value = true
  messagesError.value = ''
  try {
    messages.value = await $fetch<TenantMessage[]>(`${api}/api/bot/messages`, { credentials: 'include' })
  } catch {
    messagesError.value = t('tenantbot.messagesLoadError')
  } finally {
    messagesLoading.value = false
  }
}

function formatMessageDate(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short'
  }).format(date)
}

async function scrollConciergeToBottom() {
  await nextTick()
  if (conciergeMessageList.value) {
    conciergeMessageList.value.scrollTop = conciergeMessageList.value.scrollHeight
  }
}

async function sendConciergeMessage() {
  const message = conciergeInput.value.trim()
  if (!message || conciergeBusy.value) return

  const history = conciergeMessages.value.slice(-8)
  conciergeMessages.value.push({ role: 'user', content: message })
  conciergeInput.value = ''
  conciergeError.value = ''
  conciergeBusy.value = true
  await scrollConciergeToBottom()

  try {
    const response = await $fetch<{ reply: string }>(`${api}/api/concierge`, {
      method: 'POST',
      credentials: 'include',
      body: { message, history }
    })
    conciergeMessages.value.push({ role: 'assistant', content: response.reply })
  } catch {
    conciergeError.value = t('concierge.error')
  } finally {
    conciergeBusy.value = false
    await scrollConciergeToBottom()
  }
}

const cards = [
  { key: 'leads', icon: '◎' },
  { key: 'outreach', icon: '✎' },
  { key: 'billing', icon: '❖' }
]
</script>

<template>
  <main class="app-wrap relative min-h-dvh px-6 py-16">
    <div class="site-grid" aria-hidden="true" />
    <div class="relative z-10 mx-auto max-w-4xl">
      <div class="flex items-center justify-between border-b border-line pb-6">
        <NuxtLink :to="localePath('/')" class="brand-mark flex items-center gap-2 text-lg font-extrabold tracking-tight"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></NuxtLink>
        <NuxtLink :to="localePath('/account')" class="text-sm text-dim transition hover:text-gold">{{ $t('auth.account.title') }}</NuxtLink>
      </div>

      <div v-if="loading" class="py-24 text-center text-dim" aria-live="polite">…</div>

      <section v-else-if="me?.verified === false" class="glass-card mx-auto mt-16 max-w-xl rounded-3xl border-t-2 border-t-gold p-8 text-center sm:p-12">
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full border border-gold/40 bg-gold/10 text-xl text-gold" aria-hidden="true">✉</div>
        <h1 class="mt-6 text-2xl font-extrabold">{{ $t('onboarding.verifyGateTitle') }}</h1>
        <p class="mx-auto mt-3 max-w-md text-dim">{{ $t('onboarding.verifyGateBody') }}</p>
      </section>

      <template v-else>
        <h1 class="mt-10 text-3xl font-extrabold">
          <span class="text-gradient">{{ $t('auth.app.greeting') }}</span>
          <span v-if="me?.name">, {{ me.name }}</span>
        </h1>
        <p class="mt-2 text-dim">{{ $t('auth.app.subtitle') }}</p>

        <section v-if="hasProfile === false" class="glass-card mt-8 rounded-2xl border border-gold/40 p-6 sm:flex sm:items-center sm:justify-between sm:gap-8 sm:p-8">
          <div>
            <h2 class="text-xl font-extrabold text-gold">{{ $t('onboarding.setupTitle') }}</h2>
            <p class="mt-2 text-sm text-dim">{{ $t('onboarding.setupDescription') }}</p>
          </div>
          <NuxtLink :to="localePath('/onboarding')" class="mt-5 inline-flex shrink-0 rounded-full bg-brand px-6 py-3 font-bold text-night transition hover:bg-gold-deep sm:mt-0">
            {{ $t('onboarding.setupCta') }}
          </NuxtLink>
        </section>

        <div v-else-if="hasProfile" class="mt-6 text-end">
          <NuxtLink :to="localePath('/onboarding')" class="text-sm font-semibold text-gold transition hover:text-gold-deep">
            {{ $t('onboarding.editProfile') }}
          </NuxtLink>
        </div>

        <section class="glass-card concierge-card mt-10 overflow-hidden rounded-3xl">
          <button
            type="button"
            class="concierge-toggle flex w-full items-center justify-between gap-5 p-6 text-start sm:p-8"
            :aria-expanded="conciergeOpen"
            aria-controls="concierge-chat"
            @click="conciergeOpen = !conciergeOpen"
          >
            <span class="flex min-w-0 items-center gap-4">
              <span class="concierge-mark flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl text-xl text-gold" aria-hidden="true">✦</span>
              <span>
                <span class="block text-xs font-bold uppercase tracking-widest text-gold">{{ $t('concierge.eyebrow') }}</span>
                <span class="mt-2 block text-xl font-extrabold text-ivory">{{ $t('concierge.title') }}</span>
                <span class="mt-1.5 block text-sm leading-6 text-dim">{{ $t('concierge.description') }}</span>
              </span>
            </span>
            <span class="shrink-0 text-xs font-bold text-gold">
              {{ conciergeOpen ? $t('concierge.close') : $t('concierge.open') }}
            </span>
          </button>

          <div v-if="conciergeOpen" id="concierge-chat" class="concierge-panel border-t border-line px-4 pb-5 pt-4 sm:px-6 sm:pb-6">
            <div ref="conciergeMessageList" class="concierge-message-list grid max-h-96 min-h-56 gap-3 overflow-y-auto rounded-2xl p-4 sm:p-5" role="log" aria-live="polite">
              <p v-if="conciergeMessages.length === 0 && !conciergeBusy" class="m-auto max-w-md px-4 text-center text-sm leading-7 text-dim">
                {{ $t('concierge.empty') }}
              </p>
              <div v-for="(chatMessage, index) in conciergeMessages" :key="index" class="flex" :class="chatMessage.role === 'user' ? 'justify-end' : 'justify-start'">
                <div class="concierge-bubble rounded-2xl px-4 py-3" :class="`concierge-bubble--${chatMessage.role}`">
                  <p class="concierge-speaker">{{ $t(`concierge.${chatMessage.role}`) }}</p>
                  <p class="mt-1.5 whitespace-pre-wrap text-sm leading-7" dir="auto">{{ chatMessage.content }}</p>
                </div>
              </div>
              <div v-if="conciergeBusy" class="flex justify-start">
                <div class="concierge-bubble concierge-bubble--assistant rounded-2xl px-4 py-3 text-sm text-dim" role="status">
                  {{ $t('concierge.sending') }}
                </div>
              </div>
            </div>

            <form class="mt-4" @submit.prevent="sendConciergeMessage">
              <label for="concierge-message" class="sr-only">{{ $t('concierge.placeholder') }}</label>
              <textarea
                id="concierge-message"
                v-model="conciergeInput"
                rows="3"
                maxlength="2000"
                dir="auto"
                class="concierge-input w-full resize-none rounded-2xl px-4 py-3 text-sm leading-7"
                :placeholder="$t('concierge.placeholder')"
                @keydown.enter.exact.prevent="sendConciergeMessage"
              />
              <div class="mt-3 flex flex-wrap items-center justify-between gap-3">
                <span class="text-xs text-dim">{{ $t('concierge.inputHint') }}</span>
                <button type="submit" :disabled="conciergeBusy || !conciergeInput.trim()" class="rounded-full bg-brand px-6 py-2.5 text-sm font-bold text-night transition hover:bg-gold-deep disabled:opacity-50">
                  {{ conciergeBusy ? $t('concierge.sending') : $t('concierge.send') }}
                </button>
              </div>
            </form>
            <p v-if="conciergeError" class="mt-4 text-sm text-red-300" role="alert">{{ conciergeError }}</p>
          </div>
        </section>

        <section class="glass-card tenantbot-card mt-6 rounded-3xl p-6 sm:p-8">
          <div class="flex flex-wrap items-start justify-between gap-4">
            <div class="max-w-2xl">
              <p class="text-xs font-bold uppercase tracking-widest text-gold">TELEGRAM · 24/7</p>
              <h2 class="mt-3 text-2xl font-extrabold">{{ $t('tenantbot.connectTitle') }}</h2>
              <p class="mt-2 text-sm leading-7 text-dim">{{ $t('tenantbot.connectDescription') }}</p>
            </div>
            <span v-if="botStatus?.active" class="live-pill inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-bold text-brand2">
              <span class="live-dot h-1.5 w-1.5 rounded-full bg-brand2" aria-hidden="true" />
              {{ $t('tenantbot.active') }}
            </span>
          </div>

          <div v-if="botLoading" class="mt-8 text-sm text-dim" aria-live="polite">…</div>

          <div v-else-if="botStatus?.active" class="connected-panel mt-8 flex flex-wrap items-center justify-between gap-5 rounded-2xl p-5">
            <div>
              <p class="text-xs font-semibold text-dim">{{ $t('tenantbot.connected') }}</p>
              <p class="mt-1 font-mono text-lg font-bold text-gold" dir="ltr">{{ '@' + botStatus.bot_username }}</p>
            </div>
            <button type="button" :disabled="botBusy" class="rounded-full border border-red-400/30 px-5 py-2.5 text-sm font-bold text-red-300 transition hover:border-red-300 hover:text-red-200 disabled:opacity-50" @click="disconnectTenantBot">
              {{ botBusy ? '…' : $t('tenantbot.disconnect') }}
            </button>
          </div>

          <form v-else class="mt-8" @submit.prevent="connectTenantBot">
            <label class="block">
              <span class="text-sm font-bold text-ivory">{{ $t('tenantbot.tokenLabel') }}</span>
              <input v-model="botToken" type="password" required autocomplete="off" spellcheck="false" dir="ltr" class="bot-token-input mt-2 w-full rounded-xl px-4 py-3.5 font-mono text-sm" :placeholder="$t('tenantbot.tokenPlaceholder')" />
              <span class="mt-2 block text-xs leading-6 text-dim">{{ $t('tenantbot.tokenHelp') }}</span>
            </label>
            <button type="submit" :disabled="botBusy" class="mt-5 w-full rounded-full bg-brand px-6 py-3 font-bold text-night transition hover:bg-gold-deep disabled:opacity-50 sm:w-auto sm:min-w-44">
              {{ botBusy ? $t('tenantbot.connecting') : $t('tenantbot.connect') }}
            </button>
          </form>

          <p v-if="botError" class="mt-5 text-sm text-red-300" role="alert">{{ botError }}</p>
          <p v-if="botNotice" class="mt-5 text-sm font-semibold text-gold" role="status">{{ botNotice }}</p>
        </section>

        <section class="glass-card messages-card mt-6 rounded-3xl p-6 sm:p-8">
          <div class="flex items-start justify-between gap-5">
            <div>
              <h2 class="text-xl font-extrabold">{{ $t('tenantbot.recentTitle') }}</h2>
              <p class="mt-2 text-sm text-dim">{{ $t('tenantbot.recentDescription') }}</p>
            </div>
            <button type="button" :disabled="messagesLoading" class="shrink-0 rounded-full border border-line px-4 py-2 text-xs font-bold text-gold transition hover:border-gold/50 disabled:opacity-50" @click="loadTenantMessages">
              {{ messagesLoading ? '…' : $t('tenantbot.refresh') }}
            </button>
          </div>

          <p v-if="messagesError" class="mt-6 text-sm text-red-300" role="alert">{{ messagesError }}</p>
          <div v-else-if="messagesLoading" class="py-12 text-center text-dim" aria-live="polite">…</div>
          <div v-else-if="messages.length === 0" class="empty-messages mt-7 rounded-2xl px-5 py-10 text-center text-sm text-dim">
            {{ $t('tenantbot.empty') }}
          </div>
          <div v-else class="mt-7 grid gap-4">
            <article v-for="message in messages" :key="message.id" class="message-row rounded-2xl p-5" :class="{ 'message-row--escalated': message.escalated }">
              <div class="flex flex-wrap items-center justify-between gap-3 text-xs text-dim">
                <span class="font-semibold text-ivory">{{ message.from_name || $t('tenantbot.unknownCustomer') }}</span>
                <div class="flex items-center gap-3">
                  <span v-if="message.escalated" class="rounded-full bg-red-400/10 px-2.5 py-1 font-bold text-red-300">{{ $t('tenantbot.escalated') }}</span>
                  <time :datetime="message.created_at">{{ formatMessageDate(message.created_at) }}</time>
                </div>
              </div>
              <div class="mt-5 grid gap-3">
                <div>
                  <p class="message-label">{{ $t('tenantbot.customer') }}</p>
                  <p class="mt-1.5 whitespace-pre-wrap text-sm leading-7 text-ivory" dir="auto">{{ message.customer_text }}</p>
                </div>
                <div class="message-answer rounded-xl px-4 py-3">
                  <p class="message-label text-gold">{{ $t('tenantbot.assistant') }}</p>
                  <p class="mt-1.5 whitespace-pre-wrap text-sm leading-7 text-ivory" dir="auto">{{ message.agent_reply }}</p>
                </div>
              </div>
            </article>
          </div>
        </section>

        <div class="mt-10 grid gap-6 sm:grid-cols-3">
          <div v-for="c in cards" :key="c.key" class="glass-card coming-soon rounded-2xl p-8">
            <div class="text-2xl text-gold">{{ c.icon }}</div>
            <h2 class="mt-4 font-bold">{{ $t(`auth.app.${c.key}`) }}</h2>
            <p class="mt-1.5 text-sm text-dim">{{ $t('auth.app.soon') }}</p>
          </div>
        </div>
      </template>
    </div>
  </main>
</template>

<style scoped>
.coming-soon {
  opacity: 0.88;
}
.concierge-card {
  border-block-start: 2px solid var(--gold);
}
.concierge-toggle {
  transition: background 160ms ease;
}
.concierge-toggle:hover {
  background: rgba(212, 191, 148, 0.035);
}
.concierge-mark {
  background: rgba(212, 191, 148, 0.08);
  border: 1px solid rgba(212, 191, 148, 0.24);
  box-shadow: inset 0 0 1.5rem rgba(212, 191, 148, 0.04);
}
.concierge-panel {
  background: rgba(3, 8, 18, 0.24);
}
.concierge-message-list {
  background: rgba(20, 20, 31, 0.64);
  border: 1px solid var(--line-soft);
  scroll-behavior: smooth;
}
.concierge-bubble {
  max-width: min(88%, 38rem);
}
.concierge-bubble--assistant {
  background: rgba(212, 191, 148, 0.07);
  border: 1px solid rgba(212, 191, 148, 0.2);
  color: var(--ivory);
}
.concierge-bubble--user {
  background: linear-gradient(135deg, var(--gold), var(--gold-deep));
  color: var(--night);
}
.concierge-speaker {
  font-size: 0.65rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  opacity: 0.72;
  text-transform: uppercase;
}
.concierge-input {
  background: rgba(20, 20, 31, 0.74);
  border: 1px solid var(--line-soft);
  color: var(--ivory);
  outline: none;
  text-align: start;
  transition: border-color 160ms ease, box-shadow 160ms ease;
}
.concierge-input:focus {
  border-color: var(--gold);
  box-shadow: 0 0 0 3px rgba(212, 191, 148, 0.14);
}
.tenantbot-card {
  border-block-start: 2px solid var(--gold);
}
.live-pill {
  background: rgba(72, 202, 228, 0.08);
  border: 1px solid rgba(72, 202, 228, 0.22);
}
.live-dot {
  box-shadow: 0 0 0.8rem rgba(72, 202, 228, 0.9);
}
.connected-panel,
.empty-messages,
.message-row {
  background: rgba(20, 20, 31, 0.6);
  border: 1px solid var(--line-soft);
}
.bot-token-input {
  background: rgba(20, 20, 31, 0.74);
  border: 1px solid var(--line-soft);
  color: var(--ivory);
  outline: none;
  transition: border-color 160ms ease, box-shadow 160ms ease;
}
.bot-token-input:focus {
  border-color: var(--gold);
  box-shadow: 0 0 0 3px rgba(212, 191, 148, 0.14);
}
.message-row--escalated {
  border-color: rgba(248, 113, 113, 0.35);
}
.message-label {
  color: var(--dim);
  font-size: 0.7rem;
  font-weight: 800;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}
.message-answer {
  background: rgba(212, 191, 148, 0.055);
  border-inline-start: 2px solid rgba(212, 191, 148, 0.5);
}
:global(html[lang='fa']) .message-label,
:global(html[lang='ar']) .message-label,
:global(html[lang='fa']) .concierge-speaker,
:global(html[lang='ar']) .concierge-speaker {
  letter-spacing: 0;
}
</style>
