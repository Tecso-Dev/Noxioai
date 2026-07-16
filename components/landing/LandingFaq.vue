<script setup lang="ts">
const items = ['one', 'two', 'three', 'four']
</script>

<template>
  <section class="faq-section landing-section mx-auto max-w-3xl px-6 py-24">
    <h2 class="section-title text-gradient text-center text-3xl sm:text-4xl font-extrabold">{{ $t('faq.heading') }}</h2>
    <div class="mt-14 divide-y divide-line border-t border-line">
      <details
        v-for="(key, i) in items"
        :key="key"
        v-motion
        :initial="{ opacity: 0, y: 24 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 450, delay: i * 90 } }"
        class="faq-item group"
      >
        <summary class="flex min-h-14 cursor-pointer list-none items-center justify-between gap-4 py-5 font-semibold text-ivory">
          {{ $t(`faq.items.${key}.q`) }}
          <svg class="faq-chevron h-5 w-5 shrink-0 transition-transform duration-200 group-open:rotate-180" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path d="m6 9 6 6 6-6" />
          </svg>
        </summary>
        <p class="pb-5 text-sm text-dim max-w-prose leading-relaxed">{{ $t(`faq.items.${key}.a`) }}</p>
      </details>
    </div>
  </section>
</template>

<style scoped>
.faq-item summary::-webkit-details-marker {
  display: none;
}
.faq-chevron {
  color: var(--gold);
}
.faq-item[open] summary {
  color: var(--gold);
}
</style>
