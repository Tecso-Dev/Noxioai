<script setup lang="ts">
const team = [
  { key: 'nika', hair: '#e85d9a', shirt: '#8E2DE2' },
  { key: 'dara', hair: '#2b2b3a', shirt: '#48CAE4' },
  { key: 'sara', hair: '#a05a2c', shirt: '#22c55e' },
  { key: 'avisa', hair: '#f4c430', shirt: '#fb7185' }
]

// 3D mouse-tilt on the office panel — transform-only (GPU), disabled for reduced-motion
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
  <header class="relative overflow-hidden">
    <div class="glow absolute -top-32 inset-x-0 h-96 pointer-events-none" />
    <nav class="relative z-10 mx-auto max-w-5xl flex items-center justify-between px-6 py-6">
      <span class="text-xl font-extrabold tracking-tight">NOXIO<span class="text-brand2">AI</span></span>
      <div class="flex items-center gap-4">
        <LandingLangSwitcher />
        <a href="#waitlist" class="rounded-full bg-brand px-4 py-2 text-sm font-semibold hover:opacity-90 transition">
          {{ $t('nav.waitlist') }}
        </a>
      </div>
    </nav>

    <div class="relative z-10 mx-auto max-w-3xl px-6 pt-14 pb-10 text-center">
      <span
        v-motion :initial="{ opacity: 0, y: 16 }" :enter="{ opacity: 1, y: 0, transition: { duration: 500 } }"
        class="inline-block rounded-full border border-line bg-panel px-4 py-1.5 text-sm text-brand2 font-semibold"
      >
        {{ $t('hero.badge') }}
      </span>
      <h1
        v-motion :initial="{ opacity: 0, y: 24 }" :enter="{ opacity: 1, y: 0, transition: { duration: 600, delay: 100 } }"
        class="mt-6 text-4xl sm:text-6xl font-extrabold leading-tight"
      >
        <span class="text-gradient">{{ $t('hero.title1') }}</span><br />{{ $t('hero.title2') }}
      </h1>
      <p
        v-motion :initial="{ opacity: 0, y: 24 }" :enter="{ opacity: 1, y: 0, transition: { duration: 600, delay: 200 } }"
        class="mt-6 text-lg text-dim"
      >
        {{ $t('hero.subtitle') }}
      </p>
      <div
        v-motion :initial="{ opacity: 0, y: 24 }" :enter="{ opacity: 1, y: 0, transition: { duration: 600, delay: 300 } }"
        class="mt-8 flex flex-wrap justify-center gap-3"
      >
        <a href="#waitlist" class="rounded-full bg-brand px-6 py-3 font-bold hover:opacity-90 transition">{{ $t('hero.cta') }}</a>
        <a href="#team" class="rounded-full border border-line bg-panel px-6 py-3 font-bold hover:border-brand2 transition">{{ $t('hero.secondary') }}</a>
      </div>
    </div>

    <!-- mini pixel office preview -->
    <div
      v-motion :initial="{ opacity: 0, y: 30 }" :enter="{ opacity: 1, y: 0, transition: { duration: 700, delay: 450 } }"
      class="relative z-10 mx-auto max-w-3xl px-6 pb-16"
      style="perspective: 900px"
      @mousemove="onTilt"
      @mouseleave="resetTilt"
    >
      <div
        class="rounded-2xl border border-line bg-panel/70 backdrop-blur px-6 pt-8 pb-6 will-change-transform transition-transform duration-150 ease-out"
        :style="{ transform: `rotateX(${tilt.rx}deg) rotateY(${tilt.ry}deg)` }"
      >
        <div class="flex items-end justify-around">
          <div v-for="(m, i) in team" :key="m.key" class="flex flex-col items-center gap-1">
            <PixelPerson :hair="m.hair" :shirt="m.shirt" :scale="4" :style="{ animationDelay: i * 0.35 + 's' }" />
            <div class="h-3 w-16 rounded-sm bg-line" />
            <span class="text-xs text-dim mt-1">{{ $t(`team.members.${m.key}.name`) }}</span>
          </div>
        </div>
      </div>
    </div>
  </header>
</template>
