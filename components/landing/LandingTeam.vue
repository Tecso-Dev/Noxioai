<script setup lang="ts">
const members = [
  { key: 'nika', hair: '#e85d9a', shirt: '#8E2DE2' },
  { key: 'dara', hair: '#2b2b3a', shirt: '#48CAE4' },
  { key: 'sara', hair: '#a05a2c', shirt: '#22c55e' },
  { key: 'avisa', hair: '#f4c430', shirt: '#fb7185' }
]
</script>

<template>
  <section id="team" class="team-section landing-section mx-auto max-w-5xl px-6 py-24">
    <h2 class="section-title text-center text-3xl sm:text-4xl font-extrabold">{{ $t('team.heading') }}</h2>
    <p class="section-sub mt-3 text-center text-dim max-w-xl mx-auto">{{ $t('team.sub') }}</p>

    <div class="team-grid mt-12 grid gap-5 sm:grid-cols-2 lg:grid-cols-4">
      <div
        v-for="(m, i) in members"
        :key="m.key"
        v-motion
        :initial="{ opacity: 0, y: 32 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 120 } }"
        class="team-card card3d rounded-2xl glass-card p-6 text-center hover:border-brand2"
      >
        <div class="flex justify-center">
          <PixelPerson :hair="m.hair" :shirt="m.shirt" :scale="5" :style="{ animationDelay: i * 0.3 + 's' }" />
        </div>
        <h3 class="mt-4 text-xl font-bold">{{ $t(`team.members.${m.key}.name`) }}</h3>
        <div class="text-sm font-semibold text-brand2">{{ $t(`team.members.${m.key}.role`) }}</div>
        <p class="mt-3 text-sm text-dim">{{ $t(`team.members.${m.key}.desc`) }}</p>
      </div>
    </div>
  </section>
</template>
