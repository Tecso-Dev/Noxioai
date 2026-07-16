<script setup lang="ts">
const localePath = useLocalePath()
</script>

<template>
  <footer class="noxio-footer relative border-t border-line py-8 text-center text-sm text-dim">
    <div class="mx-auto flex max-w-6xl flex-col items-center gap-4 px-6">
      <nav class="flex flex-wrap justify-center gap-x-5 gap-y-2">
        <NuxtLink :to="localePath('/privacy')" class="transition hover:text-gold">
          {{ $t('footer.privacy') }}
        </NuxtLink>
        <NuxtLink :to="localePath('/terms')" class="transition hover:text-gold">
          {{ $t('footer.terms') }}
        </NuxtLink>
        <NuxtLink :to="localePath('/about')" class="transition hover:text-gold">
          {{ $t('footer.about') }}
        </NuxtLink>
      </nav>
      <div>
        <p>NOXIOAI — {{ $t('footer.soon') }}</p>
        <p class="mt-1">
          {{ $t('footer.made') }} ·
          <a href="https://github.com/Tecso-Dev" class="transition hover:text-gold">Tecso-Dev</a>
        </p>
      </div>
    </div>
  </footer>
</template>
