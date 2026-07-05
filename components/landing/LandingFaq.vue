<script setup lang="ts">
const items = ['one', 'two', 'three', 'four']
</script>

<template>
  <section class="mx-auto max-w-3xl px-6 py-16">
    <h2 class="text-center text-3xl sm:text-4xl font-extrabold">{{ $t('faq.heading') }}</h2>
    <div class="mt-10 space-y-3">
      <details
        v-for="(key, i) in items"
        :key="key"
        v-motion
        :initial="{ opacity: 0, y: 24 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 450, delay: i * 90 } }"
        class="faq-item group rounded-2xl border border-line bg-panel px-6 py-4"
      >
        <summary class="flex min-h-11 cursor-pointer list-none items-center justify-between gap-4 font-bold">
          {{ $t(`faq.items.${key}.q`) }}
          <svg class="h-5 w-5 shrink-0 text-brand2 transition-transform duration-200 group-open:rotate-180" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="m6 9 6 6 6-6" />
          </svg>
        </summary>
        <p class="pt-3 text-sm text-dim">{{ $t(`faq.items.${key}.a`) }}</p>
      </details>
    </div>
  </section>
</template>

<style scoped>
.faq-item summary::-webkit-details-marker {
  display: none;
}
</style>
