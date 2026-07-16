<script setup lang="ts">
const localePath = useLocalePath()
</script>

<template>
  <header class="hero relative overflow-hidden">
    <!-- single ambient visual: the fine-dot sphere, floating free — no card, no HUD chrome -->
    <div class="hero-ambient" aria-hidden="true">
      <ClientOnly><LandingHeroScene /></ClientOnly>
    </div>
    <div class="glow hero-glow absolute -top-32 inset-x-0 h-96 pointer-events-none" />
    <div class="hero-noise" aria-hidden="true" />

    <nav class="hero-nav relative z-10 mx-auto max-w-6xl flex items-center justify-between px-6 py-7">
      <span class="brand-mark flex items-center gap-2 text-xl font-extrabold tracking-tight"><img src="/brand/mark-dark.png" alt="" class="h-7 w-7 rounded-full" />NOXIO<span class="brand-accent">AI</span></span>
      <div class="flex items-center gap-4">
        <NuxtLink :to="localePath('/services')" class="nav-link text-sm font-semibold text-dim transition hover:text-snow">
          {{ $t('nav.services') }}
        </NuxtLink>
        <LandingLangSwitcher />
        <a href="#waitlist" class="nav-cta rounded-full px-4 py-2 text-sm font-semibold">
          {{ $t('nav.waitlist') }}
        </a>
      </div>
    </nav>

    <div class="hero-copy relative z-10 mx-auto max-w-3xl px-6 pt-24 pb-28 sm:pt-32 sm:pb-36 text-center">
      <span class="hero-badge inline-block rounded-full border px-4 py-1.5 text-sm font-semibold">
        {{ $t('hero.badge') }}
      </span>
      <h1 class="hero-title mt-8 text-5xl sm:text-7xl font-bold leading-[1.05]">
        <span class="hero-title-accent">{{ $t('hero.title1') }}</span><br />{{ $t('hero.title2') }}
      </h1>
      <p class="hero-subtitle mt-8 text-lg">
        {{ $t('hero.subtitle') }}
      </p>
      <div class="hero-actions mt-10 flex flex-wrap justify-center gap-3">
        <a href="#waitlist" class="primary-cta rounded-full px-6 py-3 font-bold">{{ $t('hero.cta') }}</a>
        <a href="#team" class="secondary-cta rounded-full border px-6 py-3 font-bold">{{ $t('hero.secondary') }}</a>
      </div>
    </div>
  </header>
</template>
