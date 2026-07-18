<script setup lang="ts">
type BusinessProfile = {
  business_name: string
  sells: string
  ideal_customer: string
  city: string
  country: string
  language: string
  website: string
  telegram: string
  knowledge: string
  goals: string
}

const { t } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('onboarding.title')} — NOXIOAI` })

const form = reactive<BusinessProfile>({
  business_name: '',
  sells: '',
  ideal_customer: '',
  city: '',
  country: '',
  language: '',
  website: '',
  telegram: '',
  knowledge: '',
  goals: ''
})
const ready = ref(false)
const verified = ref(false)
const busy = ref(false)
const saved = ref(false)
const err = ref('')

onMounted(async () => {
  try {
    const me = await $fetch<{ verified: boolean }>(`${api}/api/auth/me`, { credentials: 'include' })
    verified.value = me.verified
  } catch {
    await navigateTo(localePath('/login'))
    return
  }

  if (!verified.value) {
    ready.value = true
    return
  }

  try {
    const profile = await $fetch<Partial<BusinessProfile>>(`${api}/api/profile`, { credentials: 'include' })
    Object.assign(form, profile)
  } catch {
    err.value = t('onboarding.loadError')
  } finally {
    ready.value = true
  }
})

async function submit() {
  err.value = ''
  saved.value = false
  busy.value = true
  try {
    await $fetch(`${api}/api/profile`, {
      method: 'POST',
      credentials: 'include',
      body: form
    })
    saved.value = true
    await nextTick()
    await navigateTo(localePath('/app'))
  } catch {
    err.value = t('onboarding.saveError')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <main class="app-wrap relative min-h-dvh px-5 py-10 sm:px-6 sm:py-16">
    <div class="site-grid" aria-hidden="true" />
    <div class="relative z-10 mx-auto max-w-5xl">
      <header class="flex items-center justify-between border-b border-line pb-6">
        <NuxtLink :to="localePath('/')" class="brand-mark flex items-center gap-2 text-lg font-extrabold tracking-tight">
          <img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span>
        </NuxtLink>
        <NuxtLink :to="localePath('/app')" class="text-sm text-dim transition hover:text-gold">
          {{ $t('onboarding.backToApp') }}
        </NuxtLink>
      </header>

      <div v-if="!ready" class="py-24 text-center text-dim" aria-live="polite">…</div>

      <section v-else-if="!verified" class="glass-card mx-auto mt-16 max-w-xl rounded-3xl border-t-2 border-t-gold p-8 text-center sm:p-12">
        <div class="mx-auto flex h-12 w-12 items-center justify-center rounded-full border border-gold/40 bg-gold/10 text-xl text-gold" aria-hidden="true">✉</div>
        <h1 class="mt-6 text-2xl font-extrabold">{{ $t('onboarding.verifyGateTitle') }}</h1>
        <p class="mx-auto mt-3 max-w-md text-dim">{{ $t('onboarding.verifyGateBody') }}</p>
      </section>

      <template v-else>
        <div class="mx-auto mt-12 max-w-3xl text-center">
          <p class="text-xs font-bold uppercase tracking-widest text-gold">NOXIOAI</p>
          <h1 class="mt-3 text-3xl font-extrabold sm:text-4xl">
            <span class="text-gradient">{{ $t('onboarding.title') }}</span>
          </h1>
          <p class="mx-auto mt-4 max-w-2xl text-dim">{{ $t('onboarding.subtitle') }}</p>
        </div>

        <form class="glass-card onboarding-card mt-10 rounded-3xl p-6 sm:p-10" @submit.prevent="submit">
          <div class="grid gap-7 sm:grid-cols-2">
            <label class="field-block sm:col-span-2">
              <span class="field-label">{{ $t('onboarding.fields.businessName.label') }}</span>
              <input v-model="form.business_name" type="text" required autocomplete="organization" class="field-input" />
              <span class="field-help">{{ $t('onboarding.fields.businessName.helper') }}</span>
            </label>

            <label class="field-block sm:col-span-2">
              <span class="field-label">{{ $t('onboarding.fields.sells.label') }}</span>
              <textarea v-model="form.sells" rows="3" class="field-input resize-y" />
              <span class="field-help">{{ $t('onboarding.fields.sells.helper') }}</span>
            </label>

            <label class="field-block sm:col-span-2">
              <span class="field-label">{{ $t('onboarding.fields.idealCustomer.label') }}</span>
              <textarea v-model="form.ideal_customer" rows="3" class="field-input resize-y" />
              <span class="field-help">{{ $t('onboarding.fields.idealCustomer.helper') }}</span>
            </label>

            <label class="field-block">
              <span class="field-label">{{ $t('onboarding.fields.city.label') }}</span>
              <input v-model="form.city" type="text" autocomplete="address-level2" class="field-input" />
              <span class="field-help">{{ $t('onboarding.fields.city.helper') }}</span>
            </label>

            <label class="field-block">
              <span class="field-label">{{ $t('onboarding.fields.country.label') }}</span>
              <input v-model="form.country" type="text" autocomplete="country-name" class="field-input" />
              <span class="field-help">{{ $t('onboarding.fields.country.helper') }}</span>
            </label>

            <label class="field-block sm:col-span-2">
              <span class="field-label">{{ $t('onboarding.fields.language.label') }}</span>
              <input v-model="form.language" type="text" autocomplete="language" class="field-input" />
              <span class="field-help">{{ $t('onboarding.fields.language.helper') }}</span>
            </label>

            <label class="field-block">
              <span class="field-label">{{ $t('onboarding.fields.website.label') }}</span>
              <input v-model="form.website" type="url" inputmode="url" autocomplete="url" class="field-input" dir="auto" />
              <span class="field-help">{{ $t('onboarding.fields.website.helper') }}</span>
            </label>

            <label class="field-block">
              <span class="field-label">{{ $t('onboarding.fields.telegram.label') }}</span>
              <input v-model="form.telegram" type="text" autocomplete="off" class="field-input" dir="auto" />
              <span class="field-help">{{ $t('onboarding.fields.telegram.helper') }}</span>
            </label>

            <label class="field-block sm:col-span-2">
              <span class="field-label">{{ $t('onboarding.fields.knowledge.label') }}</span>
              <textarea v-model="form.knowledge" rows="10" required class="field-input knowledge-input resize-y" />
              <span class="field-help">{{ $t('onboarding.fields.knowledge.helper') }}</span>
            </label>

            <label class="field-block sm:col-span-2">
              <span class="field-label">{{ $t('onboarding.fields.goals.label') }}</span>
              <textarea v-model="form.goals" rows="4" class="field-input resize-y" />
              <span class="field-help">{{ $t('onboarding.fields.goals.helper') }}</span>
            </label>
          </div>

          <p v-if="err" class="mt-6 text-sm text-red-400" role="alert">{{ err }}</p>
          <p v-if="saved" class="mt-6 text-sm font-semibold text-gold" role="status">{{ $t('onboarding.saved') }}</p>

          <button type="submit" :disabled="busy" class="mt-8 w-full rounded-full bg-brand px-6 py-3.5 font-bold text-night transition hover:bg-gold-deep disabled:opacity-50 sm:w-auto sm:min-w-52">
            {{ busy ? '…' : $t('onboarding.submit') }}
          </button>
        </form>
      </template>
    </div>
  </main>
</template>

<style scoped>
.onboarding-card {
  border-block-start: 2px solid var(--gold);
}
.field-block,
.field-label,
.field-help {
  display: block;
}
.field-label {
  color: var(--ivory);
  font-size: 0.925rem;
  font-weight: 700;
}
.field-input {
  background: rgba(5, 10, 22, 0.72);
  border: 1px solid var(--line-soft);
  border-radius: 0.75rem;
  color: var(--ivory);
  inline-size: 100%;
  margin-block-start: 0.5rem;
  outline: none;
  padding-block: 0.8rem;
  padding-inline: 1rem;
  transition: border-color 160ms ease, box-shadow 160ms ease;
}
.field-input:focus {
  border-color: var(--gold);
  box-shadow: 0 0 0 3px rgba(212, 191, 148, 0.14);
}
.knowledge-input {
  min-block-size: 14rem;
}
.field-help {
  color: var(--dim);
  font-size: 0.78rem;
  line-height: 1.65;
  margin-block-start: 0.5rem;
}
</style>
