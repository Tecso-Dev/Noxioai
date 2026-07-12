<script setup lang="ts">
const config = useRuntimeConfig()
const email = ref('')
const company = ref('') // honeypot — real users never fill this hidden field
const state = ref<'idle' | 'sending' | 'done' | 'error'>('idle')

const hasKey = computed(() => Boolean(config.public.web3formsKey))

async function submit() {
  if (company.value) return // bot caught by honeypot
  if (!email.value || state.value === 'sending') return
  state.value = 'sending'
  try {
    const res = await fetch('https://api.web3forms.com/submit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({
        access_key: config.public.web3formsKey,
        subject: 'NOXIOAI waitlist signup',
        from_name: 'noxioai.com',
        email: email.value
      })
    })
    const data = await res.json()
    state.value = data.success ? 'done' : 'error'
  } catch {
    state.value = 'error'
  }
}
</script>

<template>
  <section id="waitlist" class="waitlist-section relative overflow-hidden py-24">
    <div class="glow absolute inset-x-0 top-0 h-72 pointer-events-none" />
    <div class="waitlist-panel glass-card rounded-3xl relative z-10 mx-auto max-w-xl px-6 py-10 text-center">
      <h2 class="text-3xl sm:text-4xl font-extrabold">{{ $t('waitlist.heading') }}</h2>
      <p class="mt-3 text-dim">{{ $t('waitlist.sub') }}</p>

      <form v-if="hasKey && state !== 'done'" class="mt-8 flex flex-col sm:flex-row gap-3" @submit.prevent="submit">
        <input v-model="company" type="text" name="company" tabindex="-1" autocomplete="off" class="hidden" aria-hidden="true" />
        <input
          v-model="email"
          type="email"
          required
          :placeholder="$t('waitlist.placeholder')"
          class="waitlist-input flex-1 rounded-full border border-line bg-panel px-5 py-3 outline-none focus:border-brand2 transition"
          dir="ltr"
        />
        <button
          type="submit"
          :disabled="state === 'sending'"
          class="waitlist-submit rounded-full bg-brand px-6 py-3 font-bold disabled:opacity-50"
        >
          {{ state === 'sending' ? $t('waitlist.sending') : $t('waitlist.button') }}
        </button>
      </form>

      <p v-if="state === 'done'" class="mt-8 rounded-2xl border border-line bg-panel px-6 py-4 font-semibold text-brand2">
        {{ $t('waitlist.success') }}
      </p>
      <p v-if="state === 'error'" class="mt-4 text-sm text-red-400">{{ $t('waitlist.error') }}</p>

      <p v-if="!hasKey" class="mt-8 rounded-2xl border border-line bg-panel px-6 py-4 text-dim">
        {{ $t('waitlist.fallback') }}
        <a href="mailto:hi@noxioai.com" class="text-brand2 font-semibold" dir="ltr">hi&#64;noxioai.com</a>
      </p>
    </div>
  </section>
</template>
