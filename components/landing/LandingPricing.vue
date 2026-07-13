<script setup lang="ts">
import { toPersianDigits } from 'parsi-text'
// Founding Members — the pre-sell offer (PRE-SELL.md). Founder prices are the
// 50%-for-life rate, billed via Stripe Checkout (Phase 4). Subscribing needs an
// account, so an anonymous visitor is routed to signup with the plan remembered.
const plans = [
  { key: 'starter', now: 25, was: 49, featured: false },
  { key: 'pro', now: 75, was: 149, featured: true },
  { key: 'agency', now: 199, was: 399, featured: false }
]
const api = useRuntimeConfig().public.apiBase
const localePath = useLocalePath()
const { locale } = useI18n()
const busy = ref('')

// Persian visitors expect Persian digits on prices (۷۵ not 75).
const price = (n: number) => (locale.value === 'fa' ? toPersianDigits(n) : String(n))

async function choosePlan(plan: string) {
  if (busy.value) return
  busy.value = plan
  try {
    // logged in → straight to Stripe Checkout; otherwise sign up first (plan remembered)
    await $fetch(`${api}/api/auth/me`, { credentials: 'include' })
    const { url } = await $fetch<{ url: string }>(`${api}/api/billing/checkout`, {
      method: 'POST', credentials: 'include', body: { plan }
    })
    window.location.href = url
  } catch {
    navigateTo(localePath(`/signup?plan=${plan}`))
  } finally {
    busy.value = ''
  }
}
</script>

<template>
  <section id="pricing" class="noxio-section relative mx-auto max-w-6xl px-6 py-20">
    <div class="text-center">
      <span class="pill inline-block rounded-full border border-line bg-panel px-4 py-1.5 text-sm text-brand2 font-semibold">
        {{ $t('pricing.spots') }}
      </span>
      <h2 v-motion :initial="{ opacity: 0, y: 20 }" :visible-once="{ opacity: 1, y: 0, transition: { duration: 600 } }"
        class="mt-5 text-3xl sm:text-5xl font-extrabold">
        <span class="text-gradient">{{ $t('pricing.heading') }}</span>
      </h2>
      <p class="mt-4 text-lg text-dim max-w-2xl mx-auto">{{ $t('pricing.sub') }}</p>
    </div>

    <div class="mt-14 grid gap-6 md:grid-cols-3">
      <div v-for="(p, i) in plans" :key="p.key"
        v-motion :initial="{ opacity: 0, y: 26 }" :visible-once="{ opacity: 1, y: 0, transition: { duration: 600, delay: i * 100 } }"
        class="glass-card relative rounded-2xl border p-7 transition"
        :class="p.featured ? 'glass-card--featured border-brand' : 'border-line'">
        <span v-if="p.featured" class="ribbon absolute -top-3 start-6 rounded-full bg-brand px-3 py-1 text-xs font-bold text-night">
          {{ $t('pricing.popular') }}
        </span>
        <h3 class="text-xl font-bold">{{ $t(`pricing.plans.${p.key}.name`) }}</h3>
        <p class="mt-1 text-sm text-dim">{{ $t(`pricing.plans.${p.key}.tagline`) }}</p>
        <div class="mt-5 flex items-end gap-2">
          <span class="text-4xl font-extrabold text-snow">€{{ price(p.now) }}</span>
          <span class="text-dim line-through mb-1">€{{ price(p.was) }}</span>
          <span class="text-dim mb-1 text-sm">{{ $t('pricing.perMonth') }}</span>
        </div>
        <span class="mt-1 inline-block text-xs text-brand2 font-semibold">{{ $t('pricing.founder') }}</span>
        <ul class="mt-6 space-y-2 text-sm">
          <li v-for="n in 4" :key="n" class="flex items-start gap-2">
            <span class="text-brand2 mt-0.5">✦</span>
            <span class="text-dim">{{ $t(`pricing.plans.${p.key}.f${n}`) }}</span>
          </li>
        </ul>
        <button type="button" :disabled="busy === p.key" @click="choosePlan(p.key)"
          class="cta mt-7 block w-full rounded-full px-5 py-3 text-center font-bold transition disabled:opacity-60"
          :class="p.featured ? 'bg-brand text-night' : 'border border-line bg-panel'">
          {{ busy === p.key ? '…' : $t('pricing.cta') }}
        </button>
      </div>
    </div>
    <p class="mt-8 text-center text-sm text-dim">{{ $t('pricing.note') }}</p>
  </section>
</template>

<style scoped>
.glass-card {
  background: rgba(7, 24, 39, 0.55);
  backdrop-filter: blur(14px);
  -webkit-backdrop-filter: blur(14px);
}
.glass-card:hover { transform: translateY(-4px); box-shadow: 0 20px 50px rgba(0, 0, 0, .4), 0 0 24px rgba(62, 225, 255, .12); }
.glass-card--featured {
  background: rgba(10, 40, 60, 0.6);
  box-shadow: 0 0 30px rgba(62, 225, 255, .18);
}
</style>
