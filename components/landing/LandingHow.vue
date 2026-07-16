<script setup lang="ts">
const steps = ['one', 'two', 'three']
</script>

<template>
  <section class="how-section landing-section mx-auto max-w-4xl px-6 py-24">
    <h2 class="section-title text-gradient text-center text-3xl sm:text-4xl font-extrabold">{{ $t('how.heading') }}</h2>
    <ol class="how-grid mt-14 grid gap-6 sm:grid-cols-3">
      <li
        v-for="(s, i) in steps"
        :key="s"
        v-motion
        :initial="{ opacity: 0, y: 32 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 140 } }"
        class="how-card rounded-2xl glass-card p-7 max-w-prose"
      >
        <span class="step-index flex h-9 w-9 items-center justify-center rounded-full bg-brand font-extrabold">{{ i + 1 }}</span>
        <h3 class="mt-5 text-lg font-bold text-ivory">{{ $t(`how.steps.${s}.title`) }}</h3>
        <p class="mt-3 text-sm text-dim leading-relaxed">{{ $t(`how.steps.${s}.desc`) }}</p>
      </li>
    </ol>
  </section>
</template>
