<script setup lang="ts">
import { toPersianDigits } from 'parsi-text'

// Noxio Autopilot — fixed-price automation offer page. Same web3forms contact
// pattern as LandingWaitlist.vue (no backend endpoint, no Stripe).
const { t, locale } = useI18n()
const localePath = useLocalePath()
const config = useRuntimeConfig()

useSeoMeta({
  title: () => t('services.meta.title'),
  description: () => t('services.meta.description'),
  ogTitle: () => t('services.meta.title'),
  ogDescription: () => t('services.meta.description'),
  ogType: 'website'
})

// Persian visitors expect Persian digits on prices (۴۹۰ not 490).
const price = (n: number) => (locale.value === 'fa' ? toPersianDigits(n) : String(n))

const tiers = [
  { key: 'starter', amount: 490, period: 'once', featured: false },
  { key: 'business', amount: 1490, period: 'once', featured: true },
  { key: 'care', amount: 75, period: 'month', featured: false }
] as const

const formEl = ref<HTMLElement | null>(null)
const selectedTier = ref<string>('business')
function pickTier(key: string) {
  selectedTier.value = key
  formEl.value?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

const name = ref('')
const email = ref('')
const message = ref('')
const company = ref('') // honeypot — real users never fill this hidden field
const state = ref<'idle' | 'sending' | 'done' | 'error'>('idle')
const hasKey = computed(() => Boolean(config.public.web3formsKey))

async function submit() {
  if (company.value) return // bot caught by honeypot
  if (!name.value || !email.value || state.value === 'sending') return
  state.value = 'sending'
  try {
    const res = await fetch('https://api.web3forms.com/submit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({
        access_key: config.public.web3formsKey,
        subject: 'Noxio Autopilot request',
        from_name: 'noxioai.com',
        name: name.value,
        email: email.value,
        message: message.value,
        plan: selectedTier.value
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
  <main class="services-page relative overflow-hidden">
    <div class="site-grid" aria-hidden="true" />
    <div class="site-glow site-glow--one" aria-hidden="true" />
    <div class="site-glow site-glow--two" aria-hidden="true" />

    <nav class="relative z-10 mx-auto max-w-6xl flex items-center justify-between px-6 py-6">
      <NuxtLink :to="localePath('/')" class="brand-mark text-xl font-extrabold tracking-tight">NOXIO<span class="text-brand2">AI</span></NuxtLink>
      <LandingLangSwitcher />
    </nav>

    <section class="relative z-10 mx-auto max-w-3xl px-6 pt-10 pb-6 text-center">
      <h1 class="text-4xl sm:text-6xl font-extrabold leading-tight">
        <span class="text-gradient">{{ $t('services.hero.title') }}</span>
      </h1>
      <p class="mt-4 text-xl text-snow font-semibold">{{ $t('services.hero.tagline') }}</p>
      <p class="mt-4 text-lg text-dim">{{ $t('services.hero.sub') }}</p>
    </section>

    <section class="noxio-section relative z-10 mx-auto max-w-6xl px-6 py-12">
      <div class="grid gap-6 md:grid-cols-3">
        <div
          v-for="tier in tiers" :key="tier.key"
          class="glass-card relative flex flex-col rounded-2xl border p-7 transition"
          :class="tier.featured ? 'glass-card--featured border-brand' : 'border-line'"
        >
          <span v-if="tier.featured" class="ribbon absolute -top-3 start-6 rounded-full bg-brand px-3 py-1 text-xs font-bold text-night">
            {{ $t('services.popular') }}
          </span>
          <h2 class="text-xl font-bold">{{ $t(`services.tiers.${tier.key}.name`) }}</h2>
          <p class="mt-1 text-sm text-dim">{{ $t(`services.tiers.${tier.key}.tagline`) }}</p>
          <div class="mt-5 flex items-end gap-2">
            <span class="text-4xl font-extrabold text-snow">€{{ price(tier.amount) }}</span>
            <span class="mb-1 text-sm text-dim">{{ tier.period === 'month' ? $t('services.perMonth') : $t('services.once') }}</span>
          </div>
          <ul class="mt-6 flex-1 space-y-2 text-sm">
            <li v-for="n in 4" :key="n" class="flex items-start gap-2">
              <span class="mt-0.5 text-brand2">✦</span>
              <span class="text-dim">{{ $t(`services.tiers.${tier.key}.f${n}`) }}</span>
            </li>
          </ul>
          <button
            type="button"
            class="cta mt-7 block w-full rounded-full px-5 py-3 text-center font-bold transition"
            :class="tier.featured ? 'bg-brand text-night' : 'border border-line bg-panel'"
            @click="pickTier(tier.key)"
          >
            {{ $t('services.choose') }}
          </button>
        </div>
      </div>
      <p class="mt-8 text-center text-sm text-dim">{{ $t('services.proof') }}</p>
    </section>

    <section id="contact" ref="formEl" class="relative z-10 py-16">
      <div class="glass-card relative z-10 mx-auto max-w-xl rounded-3xl px-6 py-10 text-center">
        <h2 class="text-3xl font-extrabold sm:text-4xl">{{ $t('services.form.heading') }}</h2>
        <p class="mt-3 text-dim">{{ $t('services.form.sub') }}</p>

        <form v-if="hasKey && state !== 'done'" class="mt-8 flex flex-col gap-3 text-start" @submit.prevent="submit">
          <input v-model="company" type="text" name="company" tabindex="-1" autocomplete="off" class="hidden" aria-hidden="true" />
          <input type="hidden" name="plan" :value="selectedTier" />
          <p class="text-sm text-dim">
            {{ $t('services.form.tierLabel') }}:
            <span class="font-semibold text-snow">{{ $t(`services.tiers.${selectedTier}.name`) }}</span>
          </p>
          <input
            v-model="name" type="text" required :placeholder="$t('services.form.name')"
            class="rounded-full border border-line bg-panel px-5 py-3 outline-none transition focus:border-brand2"
          />
          <input
            v-model="email" type="email" required :placeholder="$t('services.form.email')" dir="ltr"
            class="rounded-full border border-line bg-panel px-5 py-3 outline-none transition focus:border-brand2"
          />
          <textarea
            v-model="message" rows="3" required :placeholder="$t('services.form.message')"
            class="resize-none rounded-2xl border border-line bg-panel px-5 py-3 outline-none transition focus:border-brand2"
          />
          <button
            type="submit" :disabled="state === 'sending'"
            class="rounded-full bg-brand px-6 py-3 font-bold disabled:opacity-50"
          >
            {{ state === 'sending' ? $t('services.form.sending') : $t('services.form.submit') }}
          </button>
        </form>

        <p v-if="state === 'done'" class="mt-8 rounded-2xl border border-line bg-panel px-6 py-4 font-semibold text-brand2">
          {{ $t('services.form.success') }}
        </p>
        <p v-if="state === 'error'" class="mt-4 text-sm text-red-400">{{ $t('services.form.error') }}</p>

        <p v-if="!hasKey" class="mt-8 rounded-2xl border border-line bg-panel px-6 py-4 text-dim">
          {{ $t('services.form.fallback') }}
          <a href="mailto:hi@noxioai.com" class="font-semibold text-brand2" dir="ltr">hi&#64;noxioai.com</a>
        </p>
      </div>
    </section>

    <footer class="noxio-footer relative border-t border-line py-8 text-center text-sm text-dim">
      <p>NOXIOAI — {{ $t('footer.soon') }}</p>
      <p class="mt-1">{{ $t('footer.made') }} · <a href="https://github.com/Tecso-Dev" class="transition hover:text-brand2">Tecso-Dev</a></p>
    </footer>
  </main>
</template>

<style scoped>
.glass-card--featured {
  background: rgba(10, 40, 60, 0.6);
  box-shadow: 0 0 30px rgba(62, 225, 255, .18);
}
</style>
