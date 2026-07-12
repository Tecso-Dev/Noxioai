<script setup lang="ts">
const { t } = useI18n()
const localePath = useLocalePath()
const api = useRuntimeConfig().public.apiBase
useSeoMeta({ title: () => `${t('auth.account.title')} — NOXIOAI` })

const me = ref<{ name?: string; email?: string; locale?: string } | null>(null)

onMounted(async () => {
  try {
    me.value = await $fetch(`${api}/api/auth/me`, { credentials: 'include' })
  } catch {
    await navigateTo(localePath('/login'))
  }
})

async function logout() {
  try { await $fetch(`${api}/api/auth/logout`, { method: 'POST', credentials: 'include' }) } catch {}
  await navigateTo(localePath('/login'))
}
</script>

<template>
  <main class="app-wrap relative min-h-dvh px-6 py-12">
    <div class="site-grid" aria-hidden="true" />
    <div class="relative z-10 mx-auto max-w-md">
      <NuxtLink :to="localePath('/app')" class="text-sm text-dim hover:text-brand2 transition">← NOXIOAI</NuxtLink>
      <h1 class="mt-6 text-2xl font-bold">{{ $t('auth.account.title') }}</h1>

      <div class="glass-card mt-6 rounded-2xl p-6 space-y-4">
        <div>
          <div class="text-xs text-dim uppercase tracking-wide">{{ $t('auth.account.name') }}</div>
          <div class="mt-1 font-semibold">{{ me?.name || '—' }}</div>
        </div>
        <div>
          <div class="text-xs text-dim uppercase tracking-wide">{{ $t('auth.account.email') }}</div>
          <div class="mt-1 font-semibold">{{ me?.email || '—' }}</div>
        </div>
        <div>
          <div class="text-xs text-dim uppercase tracking-wide">{{ $t('auth.account.locale') }}</div>
          <div class="mt-1 font-semibold uppercase">{{ me?.locale || '—' }}</div>
        </div>
      </div>

      <button class="mt-6 w-full rounded-full border border-line bg-panel px-5 py-3 font-bold hover:border-brand2 transition" @click="logout">
        {{ $t('auth.account.logout') }}
      </button>
    </div>
  </main>
</template>
