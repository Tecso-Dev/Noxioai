<script setup lang="ts">
const { locale, locales } = useI18n()
const switchLocalePath = useSwitchLocalePath()
</script>

<template>
  <div class="language-switcher flex items-center gap-1 rounded-full border border-line bg-panel p-1 text-sm">
    <NuxtLink
      v-for="l in locales"
      :key="l.code"
      :to="switchLocalePath(l.code)"
      class="rounded-full px-3 py-1 transition"
      :class="l.code === locale ? 'bg-brand font-semibold' : 'text-dim hover:text-snow'"
    >
      {{ l.name }}
    </NuxtLink>
  </div>
</template>
