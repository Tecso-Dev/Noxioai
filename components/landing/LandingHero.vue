<script setup lang="ts">
// 3D mouse-tilt on the office panel — transform-only (GPU), disabled for reduced-motion
const localePath = useLocalePath()
const tilt = ref({ rx: 0, ry: 0 })
let motionOk = false
onMounted(() => {
  motionOk = !window.matchMedia('(prefers-reduced-motion: reduce)').matches
})
function onTilt(e: MouseEvent) {
  if (!motionOk) return
  const r = (e.currentTarget as HTMLElement).getBoundingClientRect()
  const px = (e.clientX - r.left) / r.width - 0.5
  const py = (e.clientY - r.top) / r.height - 0.5
  tilt.value = { rx: -py * 8, ry: px * 12 }
}
function resetTilt() {
  tilt.value = { rx: 0, ry: 0 }
}
</script>

<template>
  <header class="hero relative overflow-hidden">
    <div class="glow hero-glow absolute -top-32 inset-x-0 h-96 pointer-events-none" />
    <div class="hero-noise" aria-hidden="true" />
    <nav class="hero-nav relative z-10 mx-auto max-w-6xl flex items-center justify-between px-6 py-6">
      <span class="brand-mark text-xl font-extrabold tracking-tight">NOXIO<span class="text-brand2">AI</span></span>
      <div class="flex items-center gap-4">
        <NuxtLink :to="localePath('/services')" class="nav-link text-sm font-semibold text-dim transition hover:text-snow">
          {{ $t('nav.services') }}
        </NuxtLink>
        <LandingLangSwitcher />
        <a href="#waitlist" class="nav-cta rounded-full bg-brand px-4 py-2 text-sm font-semibold">
          {{ $t('nav.waitlist') }}
        </a>
      </div>
    </nav>

    <div class="hero-copy relative z-10 mx-auto max-w-3xl px-6 pt-14 pb-10 text-center">
      <span
        v-motion :initial="{ opacity: 0, y: 16 }" :enter="{ opacity: 1, y: 0, transition: { duration: 500 } }"
        class="hero-badge inline-block rounded-full border border-line bg-panel px-4 py-1.5 text-sm text-brand2 font-semibold"
      >
        {{ $t('hero.badge') }}
      </span>
      <h1
        v-motion :initial="{ opacity: 0, y: 24 }" :enter="{ opacity: 1, y: 0, transition: { duration: 600, delay: 100 } }"
        class="hero-title mt-6 text-4xl sm:text-6xl font-extrabold leading-tight"
      >
        <span class="text-gradient">{{ $t('hero.title1') }}</span><br />{{ $t('hero.title2') }}
      </h1>
      <p
        v-motion :initial="{ opacity: 0, y: 24 }" :enter="{ opacity: 1, y: 0, transition: { duration: 600, delay: 200 } }"
        class="hero-subtitle mt-6 text-lg text-dim"
      >
        {{ $t('hero.subtitle') }}
      </p>
      <div
        v-motion :initial="{ opacity: 0, y: 24 }" :enter="{ opacity: 1, y: 0, transition: { duration: 600, delay: 300 } }"
        class="hero-actions mt-8 flex flex-wrap justify-center gap-3"
      >
        <a href="#waitlist" class="primary-cta rounded-full bg-brand px-6 py-3 font-bold">{{ $t('hero.cta') }}</a>
        <a href="#team" class="secondary-cta rounded-full border border-line bg-panel px-6 py-3 font-bold">{{ $t('hero.secondary') }}</a>
      </div>
    </div>

    <!-- Spatial office preview. The 3D data sphere is added in the next pass. -->
    <div
      v-motion :initial="{ opacity: 0, y: 30 }" :enter="{ opacity: 1, y: 0, transition: { duration: 700, delay: 450 } }"
      class="hero-stage relative z-10 mx-auto max-w-5xl px-6 pb-20"
      @mousemove="onTilt"
      @mouseleave="resetTilt"
    >
      <div
        class="hero-console deep-3d relative rounded-2xl border border-line bg-panel/90 px-6 pt-8 pb-6 will-change-transform"
        :style="{ transform: `rotateX(${tilt.rx}deg) rotateY(${tilt.ry}deg)` }"
      >
        <div class="console-grid floor-grid absolute inset-0 rounded-2xl" aria-hidden="true" />

        <div class="console-status relative z-10 flex items-center justify-between text-xs text-dim" style="transform: translateZ(38px)">
          <span class="console-status__label">{{ $t('hero.system') }}</span>
          <span class="console-status__pulse"><i /> {{ $t('hero.live') }}</span>
        </div>

        <div class="office-scene relative" style="height: 300px; transform: translateZ(30px)">
          <ClientOnly><LandingHeroScene /></ClientOnly>
        </div>

        <div class="chip-float console-chip absolute -top-5 start-4 sm:start-8" style="transform: translateZ(60px); animation-delay: 0s" aria-hidden="true">
          {{ $t('chips.nika') }}
        </div>
        <div class="chip-float console-chip absolute top-1/3 -end-2 sm:end-0 hidden sm:block" style="transform: translateZ(75px); animation-delay: 1.1s" aria-hidden="true">
          {{ $t('chips.sara') }}
        </div>
        <div class="chip-float console-chip absolute -bottom-4 start-1/3 hidden sm:block" style="transform: translateZ(50px); animation-delay: 2.2s" aria-hidden="true">
          {{ $t('chips.dara') }}
        </div>
      </div>
    </div>
  </header>
</template>
