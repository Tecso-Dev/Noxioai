<script setup lang="ts">
const props = defineProps<{ error: object }>()
const localePath = useLocalePath()
const { t } = useI18n()

const err = computed(() => props.error as { statusCode?: number; url?: string } | null)
const isNotFound = computed(() => err.value?.statusCode === 404)
const code = computed(() => err.value?.statusCode ?? 404)
const path = computed(() => err.value?.url ?? '')

function goHome() {
  clearError({ redirect: localePath('/') })
}
</script>

<template>
  <div class="error-page" dir="auto">
    <div class="error-page__grid" aria-hidden="true" />
    <div class="error-page__scanlines" aria-hidden="true" />

    <div class="error-page__content">
      <p class="error-page__eyebrow">
        <span class="error-page__pulse" aria-hidden="true" />
        {{ t('error404.eyebrow') }}
      </p>

      <h1 class="error-page__code" :data-text="String(code)">{{ code }}</h1>

      <p class="error-page__terminal">
        <span class="error-page__prompt" aria-hidden="true">&gt;</span>
        {{ isNotFound ? t('error404.terminalNotFound') : t('error404.terminalGeneric') }}
        <span class="error-page__cursor" aria-hidden="true" />
      </p>

      <h2 class="error-page__title">{{ isNotFound ? t('error404.notFoundTitle') : t('error404.genericTitle') }}</h2>
      <p class="error-page__message">{{ isNotFound ? t('error404.notFoundMessage') : t('error404.genericMessage') }}</p>

      <p v-if="path" class="error-page__path">
        <span>{{ t('error404.pathLabel') }}</span>
        <code dir="ltr">{{ path }}</code>
      </p>

      <button type="button" class="primary-cta error-page__cta rounded-full px-6 py-3 font-bold" @click="goHome">
        {{ t('error404.cta') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.error-page {
  align-items: center;
  background:
    radial-gradient(46% 46% at 78% 12%, rgba(212, 191, 148, 0.1), transparent 70%),
    radial-gradient(40% 40% at 15% 85%, rgba(72, 202, 228, 0.08), transparent 70%),
    var(--night);
  display: flex;
  justify-content: center;
  min-height: 100dvh;
  overflow: hidden;
  padding: 6rem 1.5rem;
  position: relative;
}

.error-page__grid {
  background-image:
    linear-gradient(rgba(72, 202, 228, 0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(72, 202, 228, 0.045) 1px, transparent 1px);
  background-size: 48px 48px;
  inset: 0;
  mask-image: radial-gradient(70% 60% at 50% 40%, black, transparent 85%);
  position: absolute;
}

.error-page__scanlines {
  animation: error-scan 9s linear infinite;
  background: repeating-linear-gradient(
    180deg,
    rgba(242, 239, 232, 0.035) 0px,
    rgba(242, 239, 232, 0.035) 1px,
    transparent 1px,
    transparent 3px
  );
  inset: 0;
  opacity: 0.5;
  position: absolute;
}

.error-page__content {
  max-width: 34rem;
  position: relative;
  text-align: center;
  z-index: 1;
}

.error-page__eyebrow {
  align-items: center;
  color: var(--dim);
  display: inline-flex;
  font-family: 'DM Mono', monospace;
  font-size: 0.72rem;
  gap: 0.55rem;
  letter-spacing: 0.2em;
  text-transform: uppercase;
}

.error-page__pulse {
  background: var(--cyan);
  border-radius: 50%;
  box-shadow: 0 0 0 4px var(--cyan-soft), 0 0 14px rgba(72, 202, 228, 0.7);
  flex: none;
  height: 0.4rem;
  width: 0.4rem;
}

.error-page__code {
  color: var(--ivory);
  font-family: 'DM Mono', monospace;
  font-size: clamp(5rem, 16vw, 9rem);
  font-weight: 500;
  letter-spacing: -0.02em;
  line-height: 1;
  margin: 1.5rem 0 0;
  position: relative;
}

.error-page__code::before,
.error-page__code::after {
  content: attr(data-text);
  inset: 0;
  position: absolute;
}

.error-page__code::before {
  animation: error-glitch-a 5.5s ease-in-out infinite;
  color: var(--pulse);
  mix-blend-mode: screen;
  opacity: 0.7;
}

.error-page__code::after {
  animation: error-glitch-b 6.5s ease-in-out infinite;
  color: var(--gold);
  mix-blend-mode: screen;
  opacity: 0.6;
}

.error-page__terminal {
  color: var(--dim);
  font-family: 'DM Mono', monospace;
  font-size: 0.9rem;
  letter-spacing: 0.01em;
  margin: 1.5rem 0 0;
}

.error-page__prompt {
  color: var(--cyan);
  margin-inline-end: 0.5rem;
}

.error-page__cursor {
  animation: error-blink 1.1s step-end infinite;
  background: var(--gold);
  display: inline-block;
  height: 1em;
  margin-inline-start: 0.4rem;
  vertical-align: -0.15em;
  width: 0.5em;
}

.error-page__title {
  color: var(--ivory);
  font-size: clamp(1.3rem, 3vw, 1.7rem);
  font-weight: 800;
  letter-spacing: -0.02em;
  margin: 2rem 0 0;
}

.error-page__message {
  color: var(--dim);
  line-height: 1.7;
  margin: 0.75rem auto 0;
  max-width: 28rem;
}

.error-page__path {
  align-items: center;
  border: 1px solid var(--line-soft);
  border-radius: 0.6rem;
  color: var(--dim);
  display: inline-flex;
  flex-wrap: wrap;
  font-size: 0.78rem;
  gap: 0.5rem;
  justify-content: center;
  margin: 1.75rem 0 0;
  max-width: 100%;
  padding: 0.5rem 0.85rem;
}

.error-page__path code {
  color: var(--ivory);
  font-family: 'DM Mono', monospace;
  overflow-wrap: anywhere;
}

.error-page__cta {
  margin: 2.25rem auto 0;
}

@keyframes error-scan {
  from { transform: translateY(0); }
  to { transform: translateY(48px); }
}

@keyframes error-blink {
  0%, 45% { opacity: 1; }
  50%, 100% { opacity: 0; }
}

@keyframes error-glitch-a {
  0%, 92%, 100% { clip-path: inset(0 0 0 0); transform: translate(0, 0); }
  93% { clip-path: inset(10% 0 60% 0); transform: translate(-0.06em, 0.02em); }
  95% { clip-path: inset(60% 0 5% 0); transform: translate(0.06em, -0.02em); }
  97% { clip-path: inset(30% 0 40% 0); transform: translate(-0.04em, 0); }
}

@keyframes error-glitch-b {
  0%, 90%, 100% { clip-path: inset(0 0 0 0); transform: translate(0, 0); }
  91% { clip-path: inset(70% 0 5% 0); transform: translate(0.05em, -0.02em); }
  94% { clip-path: inset(5% 0 70% 0); transform: translate(-0.05em, 0.02em); }
  98% { clip-path: inset(40% 0 30% 0); transform: translate(0.03em, 0); }
}

@media (prefers-reduced-motion: reduce) {
  .error-page__scanlines {
    animation: none;
    opacity: 0.25;
  }

  .error-page__code::before,
  .error-page__code::after {
    animation: none;
    content: none;
  }

  .error-page__cursor {
    animation: none;
    opacity: 1;
  }
}
</style>
