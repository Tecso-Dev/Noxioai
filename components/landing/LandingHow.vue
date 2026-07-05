<script setup lang="ts">
const steps = ['one', 'two', 'three']
</script>

<template>
  <section class="mx-auto max-w-4xl px-6 py-16">
    <h2 class="text-center text-3xl sm:text-4xl font-extrabold">{{ $t('how.heading') }}</h2>
    <ol class="mt-12 grid gap-5 sm:grid-cols-3">
      <li
        v-for="(s, i) in steps"
        :key="s"
        v-motion
        :initial="{ opacity: 0, y: 32 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 140 } }"
        class="rounded-2xl border border-line bg-panel p-6"
      >
        <span class="flex h-9 w-9 items-center justify-center rounded-full bg-brand font-extrabold">{{ i + 1 }}</span>
        <h3 class="mt-4 text-lg font-bold">{{ $t(`how.steps.${s}.title`) }}</h3>
        <p class="mt-2 text-sm text-dim">{{ $t(`how.steps.${s}.desc`) }}</p>
      </li>
    </ol>
  </section>
</template>
