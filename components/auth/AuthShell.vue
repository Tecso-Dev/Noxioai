<script setup lang="ts">
defineProps<{
  eyebrow: string
  title: string
  description: string
  wide?: boolean
}>()

const localePath = useLocalePath()
</script>

<template>
  <main class="auth-stage">
    <div class="auth-stage__grid" aria-hidden="true" />
    <div class="auth-stage__flare auth-stage__flare--cyan" aria-hidden="true" />
    <div class="auth-stage__flare auth-stage__flare--gold" aria-hidden="true" />

    <nav class="auth-nav" aria-label="NOXIOAI">
      <NuxtLink :to="localePath('/')" class="auth-brand" aria-label="NOXIOAI home">
        <img src="/brand/mark-dark.png" alt="" width="32" height="32" />
        <span>NOXIO<span>AI</span></span>
      </NuxtLink>
      <div class="auth-nav__status">
        <span class="auth-status-dot" aria-hidden="true" />
        {{ $t('auth.secureConnection') }}
      </div>
    </nav>

    <div class="auth-layout" :class="{ 'auth-layout--wide': wide }">
      <aside class="auth-rail" aria-labelledby="auth-security-title">
        <div>
          <div class="auth-rail__eyebrow">
            <span class="auth-status-dot" aria-hidden="true" />
            {{ $t('auth.securityRail.eyebrow') }}
          </div>
          <h2 id="auth-security-title">{{ $t('auth.securityRail.title') }}</h2>
          <p>{{ $t('auth.securityRail.description') }}</p>
        </div>

        <ul class="auth-trust-list">
          <li>
            <span class="auth-trust-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24"><path d="M12 3 5 6v5c0 4.6 2.8 8 7 10 4.2-2 7-5.4 7-10V6l-7-3Z"/><path d="m9.5 12 1.7 1.7 3.6-4"/></svg>
            </span>
            <span><strong>{{ $t('auth.securityRail.passkeysTitle') }}</strong><small>{{ $t('auth.securityRail.passkeysText') }}</small></span>
          </li>
          <li>
            <span class="auth-trust-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24"><rect x="5" y="10" width="14" height="10" rx="2"/><path d="M8 10V7a4 4 0 0 1 8 0v3M12 14v2"/></svg>
            </span>
            <span><strong>{{ $t('auth.securityRail.sessionTitle') }}</strong><small>{{ $t('auth.securityRail.sessionText') }}</small></span>
          </li>
          <li>
            <span class="auth-trust-icon" aria-hidden="true">
              <svg viewBox="0 0 24 24"><path d="M4 12h16M12 4v16"/><circle cx="12" cy="12" r="9"/></svg>
            </span>
            <span><strong>{{ $t('auth.securityRail.privacyTitle') }}</strong><small>{{ $t('auth.securityRail.privacyText') }}</small></span>
          </li>
        </ul>

        <div class="auth-rail__signal" aria-hidden="true">
          <span>NOXIO / IDENTITY</span>
          <span class="auth-signal-line" />
          <span>01</span>
        </div>
      </aside>

      <section class="auth-surface">
        <header class="auth-surface__header">
          <p>{{ eyebrow }}</p>
          <h1>{{ title }}</h1>
          <div>{{ description }}</div>
        </header>
        <slot />
      </section>
    </div>

    <footer class="auth-footer">
      <span>© 2026 NOXIOAI</span>
      <span>{{ $t('auth.securityRail.footer') }}</span>
    </footer>
  </main>
</template>

<style scoped>
.auth-stage {
  align-items: center;
  background: #040914;
  color: var(--ivory);
  display: flex;
  isolation: isolate;
  justify-content: center;
  min-block-size: 100dvh;
  overflow: hidden;
  padding: 7rem 1.5rem 4rem;
  position: relative;
}
.auth-stage__grid {
  background-image: linear-gradient(rgba(72, 202, 228, .035) 1px, transparent 1px), linear-gradient(90deg, rgba(72, 202, 228, .035) 1px, transparent 1px);
  background-size: 48px 48px;
  inset: 0;
  mask-image: linear-gradient(to bottom, #000, transparent 85%);
  pointer-events: none;
  position: absolute;
  z-index: -3;
}
.auth-stage__grid::after {
  background: linear-gradient(90deg, transparent, rgba(72, 202, 228, .35), transparent);
  block-size: 1px;
  content: '';
  inset: 33% 0 auto;
  opacity: .35;
  position: absolute;
}
.auth-stage__flare { border-radius: 999px; filter: blur(100px); opacity: .16; pointer-events: none; position: absolute; z-index: -2; }
.auth-stage__flare--cyan { background: var(--cyan); block-size: 25rem; inline-size: 25rem; inset-block-start: -13rem; inset-inline-start: 18%; }
.auth-stage__flare--gold { background: var(--gold); block-size: 20rem; inline-size: 20rem; inset-block-end: -12rem; inset-inline-end: 12%; opacity: .1; }
.auth-nav {
  align-items: center;
  display: flex;
  inset: 0 0 auto;
  justify-content: space-between;
  padding: 1.35rem clamp(1.5rem, 5vw, 5rem);
  position: absolute;
}
.auth-brand { align-items: center; color: var(--ivory); display: inline-flex; font-size: 1rem; font-weight: 800; gap: .65rem; letter-spacing: -.04em; text-decoration: none; }
.auth-brand img { block-size: 2rem; border: 1px solid rgba(212, 191, 148, .28); border-radius: 50%; inline-size: 2rem; }
.auth-brand span span { color: var(--gold); }
.auth-nav__status { align-items: center; color: #aab5c5; display: flex; font-family: 'DM Mono', monospace; font-size: .67rem; gap: .55rem; letter-spacing: .08em; text-transform: uppercase; }
.auth-status-dot { background: #59d4aa; border-radius: 50%; box-shadow: 0 0 0 4px rgba(89, 212, 170, .1), 0 0 16px rgba(89, 212, 170, .45); display: inline-block; flex: 0 0 auto; block-size: .42rem; inline-size: .42rem; }
.auth-layout {
  background: rgba(5, 15, 29, .76);
  border: 1px solid rgba(156, 185, 213, .15);
  box-shadow: 0 36px 100px rgba(0, 0, 0, .48), inset 0 1px rgba(255, 255, 255, .025);
  display: grid;
  grid-template-columns: minmax(17rem, .72fr) minmax(22rem, 1fr);
  inline-size: min(62rem, 100%);
  min-block-size: 36rem;
  position: relative;
}
.auth-layout::before, .auth-layout::after { border-color: rgba(72, 202, 228, .5); border-style: solid; content: ''; block-size: .75rem; inline-size: .75rem; pointer-events: none; position: absolute; z-index: 3; }
.auth-layout::before { border-width: 1px 0 0 1px; inset: -.25rem auto auto -.25rem; }
.auth-layout::after { border-width: 0 1px 1px 0; inset: auto -.25rem -.25rem auto; }
.auth-layout--wide { inline-size: min(68rem, 100%); }
.auth-rail {
  background: linear-gradient(155deg, rgba(13, 35, 58, .82), rgba(5, 17, 32, .92));
  border-inline-end: 1px solid rgba(156, 185, 213, .13);
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding: clamp(2rem, 4vw, 3.4rem);
  position: relative;
}
.auth-rail::after { background: radial-gradient(circle at 50% 40%, rgba(72, 202, 228, .11), transparent 56%); content: ''; inset: 0; pointer-events: none; position: absolute; }
.auth-rail > * { position: relative; z-index: 1; }
.auth-rail__eyebrow { align-items: center; color: #9fb3c8; display: flex; font-family: 'DM Mono', monospace; font-size: .66rem; gap: .6rem; letter-spacing: .1em; text-transform: uppercase; }
.auth-rail h2 { font-size: clamp(1.65rem, 3vw, 2.35rem); letter-spacing: -.045em; line-height: 1.08; margin: 1.35rem 0 .9rem; max-inline-size: 12ch; }
.auth-rail > div > p { color: #92a1b5; font-size: .86rem; line-height: 1.7; margin: 0; max-inline-size: 30ch; }
.auth-trust-list { display: grid; gap: 1.25rem; list-style: none; margin: 3rem 0; padding: 0; }
.auth-trust-list li { align-items: flex-start; display: flex; gap: .9rem; }
.auth-trust-icon { align-items: center; background: rgba(72, 202, 228, .07); border: 1px solid rgba(72, 202, 228, .18); display: inline-flex; flex: 0 0 auto; justify-content: center; block-size: 2.15rem; inline-size: 2.15rem; }
.auth-trust-icon svg { fill: none; stroke: var(--cyan); stroke-linecap: round; stroke-linejoin: round; stroke-width: 1.4; block-size: 1.1rem; inline-size: 1.1rem; }
.auth-trust-list strong, .auth-trust-list small { display: block; }
.auth-trust-list strong { color: #dce7f0; font-size: .8rem; font-weight: 600; }
.auth-trust-list small { color: #7f91a5; font-size: .7rem; line-height: 1.5; margin-block-start: .2rem; }
.auth-rail__signal { align-items: center; color: #718399; display: flex; font-family: 'DM Mono', monospace; font-size: .58rem; gap: .65rem; letter-spacing: .08em; }
.auth-signal-line { background: linear-gradient(90deg, rgba(72, 202, 228, .5), transparent); block-size: 1px; flex: 1; }
.auth-surface { background: rgba(4, 12, 24, .68); padding: clamp(2rem, 5vw, 3.8rem); }
.auth-surface__header > p { color: var(--gold); font-family: 'DM Mono', monospace; font-size: .65rem; letter-spacing: .12em; margin: 0 0 .8rem; text-transform: uppercase; }
.auth-surface__header h1 { color: #f2f5f8; font-size: clamp(1.75rem, 3.5vw, 2.35rem); letter-spacing: -.045em; line-height: 1.12; margin: 0; }
.auth-surface__header > div { color: #8f9caf; font-size: .85rem; line-height: 1.65; margin-block-start: .75rem; max-inline-size: 45ch; }
.auth-footer { bottom: 1.2rem; color: #718399; display: flex; font-family: 'DM Mono', monospace; font-size: .6rem; gap: 2rem; justify-content: space-between; left: clamp(1.5rem, 5vw, 5rem); letter-spacing: .08em; position: absolute; right: clamp(1.5rem, 5vw, 5rem); text-transform: uppercase; }
@media (max-width: 760px) {
  .auth-stage { align-items: flex-start; padding-block: 6rem 5rem; }
  .auth-layout { grid-template-columns: 1fr; }
  .auth-rail { border-block-end: 1px solid rgba(156, 185, 213, .13); border-inline-end: 0; min-block-size: auto; padding: 1.4rem; }
  .auth-rail h2, .auth-rail > div > p, .auth-trust-list, .auth-rail__signal { display: none; }
  .auth-surface { padding: 2rem 1.35rem 2.3rem; }
  .auth-nav__status, .auth-footer span:last-child { display: none; }
}
@media (prefers-reduced-motion: reduce) {
  .auth-status-dot { box-shadow: none; }
}
</style>
