<script setup lang="ts">
const items = [
  { key: 'office', icon: '🏢' },
  { key: 'brain', icon: '🧠' },
  { key: 'persian', icon: '🇮🇷' },
  { key: 'channels', icon: '📱' }
]
</script>

<template>
  <section class="mx-auto max-w-5xl px-6 py-16">
    <h2 class="text-center text-3xl sm:text-4xl font-extrabold">{{ $t('features.heading') }}</h2>
    <div class="mt-12 grid gap-5 sm:grid-cols-2">
      <div
        v-for="(f, i) in items"
        :key="f.key"
        v-motion
        :initial="{ opacity: 0, y: 32 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 100 } }"
        class="rounded-2xl border border-line bg-panel p-6 flex gap-4 items-start"
      >
        <span class="text-3xl">{{ f.icon }}</span>
        <div>
          <h3 class="text-lg font-bold">{{ $t(`features.items.${f.key}.title`) }}</h3>
          <p class="mt-2 text-sm text-dim">{{ $t(`features.items.${f.key}.desc`) }}</p>
        </div>
      </div>
    </div>
  </section>
</template>
