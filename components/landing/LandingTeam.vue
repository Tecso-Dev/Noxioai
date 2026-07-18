<script setup lang="ts">
// palette-harmonized: charcoal hair + one accent shirt each from the
// premium-tech system (gold / cyan-pulse / ivory / gold-deep) — no neon.
const members = [
  { key: 'nika', hair: '#2b2b3a', shirt: '#d4bf94' },
  { key: 'dara', hair: '#2b2b3a', shirt: '#48CAE4' },
  { key: 'sara', hair: '#b39868', shirt: '#f2efe8' },
  { key: 'avisa', hair: '#2b2b3a', shirt: '#b39868' }
]
</script>

<template>
  <section id="team" class="team-section landing-section mx-auto max-w-5xl px-6 py-24">
    <h2 class="section-title text-gradient text-center text-3xl sm:text-4xl font-extrabold">{{ $t('team.heading') }}</h2>
    <p class="section-sub mt-3 text-center text-dim max-w-xl mx-auto">{{ $t('team.sub') }}</p>

    <div class="team-grid mt-14 grid gap-6 sm:grid-cols-2 lg:grid-cols-4">
      <div
        v-for="(m, i) in members"
        :key="m.key"
        v-motion
        :initial="{ opacity: 0, y: 32 }"
        :visible-once="{ opacity: 1, y: 0, transition: { duration: 500, delay: i * 120 } }"
        class="team-card team-card--refined rounded-2xl glass-card p-8 text-center"
      >
        <div class="flex justify-center">
          <PixelPerson :hair="m.hair" :shirt="m.shirt" :scale="5" :style="{ animationDelay: i * 0.3 + 's' }" />
        </div>
        <h3 class="mt-5 text-xl font-semibold text-ivory tracking-tight">{{ $t(`team.members.${m.key}.name`) }}</h3>
        <div class="mt-1 text-xs font-semibold uppercase tracking-wide text-dim">{{ $t(`team.members.${m.key}.role`) }}</div>
        <p class="mt-3 text-sm text-dim leading-relaxed">{{ $t(`team.members.${m.key}.desc`) }}</p>
      </div>
    </div>
  </section>
</template>

<style scoped>
.team-section .team-card--refined {
  border-color: rgba(212, 191, 148, 0.22);
}
</style>
