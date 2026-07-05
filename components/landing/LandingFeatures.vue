<script setup lang="ts">
const items = ['office', 'brain', 'persian', 'channels'] as const
</script>

<template>
  <section class="mx-auto max-w-5xl px-6 py-16">
    <h2 class="text-center text-3xl sm:text-4xl font-extrabold">{{ $t('features.heading') }}</h2>
    <div class="mt-12 grid gap-5 sm:grid-cols-2">
      <div
        v-for="(key, i) in items"
        :key="key"
        v-motion
        :initial="{ opacity: 0, y: 32 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 100 } }"
        class="card3d rounded-2xl border border-line bg-panel p-6 flex gap-4 items-start"
      >
        <span class="mt-1 flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-brand/20 text-brand2">
          <!-- SVG icons (no emoji) — stroke style, one family -->
          <svg v-if="key === 'office'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M6 22V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v18" /><path d="M2 22h20" />
            <path d="M9 7h2M13 7h2M9 11h2M13 11h2M9 15h2M13 15h2" />
          </svg>
          <svg v-else-if="key === 'brain'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <rect x="5" y="5" width="14" height="14" rx="2" /><rect x="9" y="9" width="6" height="6" rx="1" />
            <path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3" />
          </svg>
          <svg v-else-if="key === 'persian'" class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <circle cx="12" cy="12" r="9" /><path d="M3 12h18" /><path d="M12 3a15 15 0 0 1 0 18a15 15 0 0 1 0-18" />
          </svg>
          <svg v-else class="h-6 w-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="M22 2 11 13" /><path d="M22 2 15 22l-4-9-9-4Z" />
          </svg>
        </span>
        <div>
          <h3 class="text-lg font-bold">{{ $t(`features.items.${key}.title`) }}</h3>
          <p class="mt-2 text-sm text-dim">{{ $t(`features.items.${key}.desc`) }}</p>
        </div>
      </div>
    </div>
  </section>
</template>
