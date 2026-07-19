<script setup lang="ts">
const codeColumns = [
  [
    'const office = await noxio.create({',
    '  brain: businessContext,',
    "  language: 'auto',",
    '  approval: true',
    '})',
    '',
    'office.assign({',
    "  agent: 'Nika',",
    "  mission: 'launch campaign'",
    '})',
    '',
    'await office.run()',
    '',
    "office.on('mission:blocked', async (m) => {",
    '  await telegram.notifyOwner(m)',
    '})'
  ],
  [
    'type Mission = {',
    '  intent: string',
    '  context: BusinessBrain',
    '  owner: Agent',
    '  requiresApproval: boolean',
    '}',
    '',
    'const result = await team.route(',
    '  mission,',
    '  { memory: true }',
    ')',
    '',
    'if (result.needsApproval) {',
    '  await gate.wait(result.id)',
    '}'
  ],
  [
    "stream.on('agent:ready', ({ agent }) => {",
    '  workspace.update(agent)',
    '  activity.push({',
    '    state: agent.state,',
    '    timestamp: Date.now()',
    '  })',
    '})',
    '',
    "stream.on('agent:error', (err) => {",
    '  retry.schedule(err.taskId, {',
    "    backoff: 'exponential'",
    '  })',
    '})',
    '',
    'export default workspace'
  ],
  [
    'class TenantContext {',
    '  constructor(tenantId) {',
    '    this.tenantId = tenantId',
    '    this.scope = isolate(tenantId)',
    '  }',
    '',
    '  async run(task) {',
    '    const ctx = this.scope.enter()',
    '    return task.execute(ctx)',
    '  }',
    '}',
    '',
    'const auth = await verify(token, {',
    '  constantTime: true',
    '})'
  ],
  [
    'async function draft(lead) {',
    '  const brief = await atlas.analyze(lead)',
    '  const copy = await deepseek.write(brief)',
    '',
    '  return {',
    '    lead,',
    '    copy,',
    "    status: 'pending_approval'",
    '  }',
    '}',
    '',
    "bot.on('approve', async (id) => {",
    '  await queue.publish(id)',
    '})'
  ]
]

// duplicated once so the track can loop seamlessly at translateX(-50%)
const loopedCodeColumns = [...codeColumns, ...codeColumns]

const surfaces = [
  { key: 'command', image: '/showcase/command-center.webp' },
  { key: 'dossier', image: '/showcase/agent-dossier.webp' }
] as const
</script>

<template>
  <section class="showcase-section landing-section py-24" aria-labelledby="showcase-title">
    <div class="showcase-heading mx-auto max-w-3xl px-6 text-center">
      <p class="showcase-eyebrow">{{ $t('showcase.eyebrow') }}</p>
      <h2 id="showcase-title" class="section-title text-gradient mt-4 text-3xl font-extrabold sm:text-4xl">
        {{ $t('showcase.heading') }}
      </h2>
      <p class="section-sub mx-auto mt-4 max-w-2xl text-dim">
        {{ $t('showcase.sub') }}
      </p>
    </div>

    <div
      v-motion
      :initial="{ opacity: 0, y: 30 }"
      :visible-once="{ opacity: 1, y: 0, transition: { duration: 700 } }"
      class="showcase-stage"
    >
      <div class="showcase-stage__topbar">
        <span class="showcase-stage__identity">
          <span class="showcase-stage__pulse" aria-hidden="true" />
          NOXIO / WORKSTREAM
        </span>
        <span class="showcase-stage__status">{{ $t('showcase.status') }}</span>
      </div>

      <div class="showcase-code" aria-hidden="true">
        <div class="showcase-code__track">
          <div
            v-for="(column, columnIndex) in loopedCodeColumns"
            :key="columnIndex"
            class="showcase-code__column"
          >
            <code v-for="(line, lineIndex) in column" :key="lineIndex" :class="{ 'is-accent': line.includes('agent') || line.includes('mission') || line.includes('office') }">{{ line || ' ' }}</code>
          </div>
        </div>
      </div>

      <div class="showcase-seam" aria-hidden="true">
        <span class="showcase-seam__spark showcase-seam__spark--one" />
        <span class="showcase-seam__spark showcase-seam__spark--two" />
        <span class="showcase-seam__spark showcase-seam__spark--three" />
      </div>

      <div class="showcase-products">
        <div class="showcase-surfaces" role="list" tabindex="0" aria-labelledby="showcase-title">
          <article
            v-for="(surface, index) in surfaces"
            :key="surface.key"
            v-motion
            :initial="{ opacity: 0, y: 24 }"
            :visible-once="{ opacity: 1, y: 0, transition: { duration: 520, delay: 180 + index * 120 } }"
            class="product-surface"
            role="listitem"
          >
            <div class="product-surface__visual">
              <img
                :src="surface.image"
                :alt="$t(`showcase.cards.${surface.key}.alt`)"
                width="1200"
                height="754"
                loading="lazy"
                decoding="async"
              />
              <span class="product-surface__signal" aria-hidden="true" />
            </div>
            <div class="product-surface__copy" dir="auto">
              <span class="product-surface__kicker">{{ $t(`showcase.cards.${surface.key}.kicker`) }}</span>
              <h3>{{ $t(`showcase.cards.${surface.key}.title`) }}</h3>
              <p>{{ $t(`showcase.cards.${surface.key}.desc`) }}</p>
            </div>
          </article>

          <article
            v-motion
            :initial="{ opacity: 0, y: 24 }"
            :visible-once="{ opacity: 1, y: 0, transition: { duration: 520, delay: 420 } }"
            class="product-surface product-surface--feed"
            role="listitem"
          >
            <div class="mission-feed" dir="auto">
              <div class="mission-feed__header">
                <span>{{ $t('showcase.cards.feed.kicker') }}</span>
                <span class="mission-feed__live">{{ $t('showcase.cards.feed.live') }}</span>
              </div>
              <div class="mission-feed__metric">
                <strong>12</strong>
                <span>{{ $t('showcase.cards.feed.metric') }}</span>
              </div>
              <ul>
                <li>
                  <span class="mission-feed__agent">N</span>
                  <span>{{ $t('chips.nika') }}</span>
                </li>
                <li>
                  <span class="mission-feed__agent mission-feed__agent--gold">S</span>
                  <span>{{ $t('chips.sara') }}</span>
                </li>
                <li>
                  <span class="mission-feed__agent mission-feed__agent--blue">D</span>
                  <span>{{ $t('chips.dara') }}</span>
                </li>
              </ul>
            </div>
            <div class="product-surface__copy" dir="auto">
              <span class="product-surface__kicker">{{ $t('showcase.cards.feed.kicker') }}</span>
              <h3>{{ $t('showcase.cards.feed.title') }}</h3>
              <p>{{ $t('showcase.cards.feed.desc') }}</p>
            </div>
          </article>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.showcase-section {
  overflow: hidden;
}

.showcase-eyebrow {
  align-items: center;
  color: var(--cyan);
  display: inline-flex;
  font-family: 'DM Mono', monospace;
  font-size: 0.7rem;
  font-weight: 500;
  gap: 0.7rem;
  letter-spacing: 0.18em;
  text-transform: uppercase;
}

.showcase-eyebrow::before,
.showcase-eyebrow::after {
  background: linear-gradient(90deg, transparent, rgba(72, 202, 228, 0.7));
  content: '';
  height: 1px;
  width: 2rem;
}

.showcase-eyebrow::after {
  background: linear-gradient(90deg, rgba(72, 202, 228, 0.7), transparent);
}

.showcase-stage {
  background:
    linear-gradient(90deg, rgba(4, 12, 26, 0.96) 0 41%, rgba(4, 15, 29, 0.92) 48%, rgba(7, 20, 37, 0.98) 100%);
  border-block: 1px solid rgba(72, 202, 228, 0.17);
  box-shadow: 0 30px 90px rgba(0, 0, 0, 0.3), inset 0 1px rgba(255, 255, 255, 0.025);
  direction: ltr;
  margin: 3.5rem auto 0;
  max-width: 92rem;
  min-height: 34rem;
  overflow: hidden;
  position: relative;
}

.showcase-stage::after {
  background-image: radial-gradient(rgba(72, 202, 228, 0.22) 0.6px, transparent 0.7px);
  background-size: 10px 10px;
  content: '';
  inset: 0 0 0 41%;
  mask-image: linear-gradient(90deg, black, transparent 72%);
  opacity: 0.2;
  pointer-events: none;
  position: absolute;
}

.showcase-stage__topbar {
  align-items: center;
  background: rgba(3, 10, 22, 0.74);
  border-bottom: 1px solid rgba(72, 202, 228, 0.11);
  color: rgba(242, 239, 232, 0.5);
  display: flex;
  font-family: 'DM Mono', monospace;
  font-size: 0.62rem;
  inset: 0 0 auto;
  justify-content: space-between;
  letter-spacing: 0.17em;
  min-height: 2.75rem;
  padding: 0 1.5rem;
  position: absolute;
  text-transform: uppercase;
  z-index: 8;
}

.showcase-stage__identity {
  align-items: center;
  display: inline-flex;
  gap: 0.6rem;
}

.showcase-stage__pulse {
  background: var(--cyan);
  border-radius: 50%;
  box-shadow: 0 0 0 4px var(--cyan-soft), 0 0 14px rgba(72, 202, 228, 0.8);
  height: 0.38rem;
  width: 0.38rem;
}

.showcase-stage__status {
  color: rgba(212, 191, 148, 0.78);
}

.showcase-code {
  inset: 2.75rem 59% 0 0;
  mask-image: linear-gradient(90deg, black 0%, black 68%, transparent 100%);
  overflow: hidden;
  padding: 2rem 0 1.5rem 1.5rem;
  position: absolute;
}

.showcase-code__track {
  animation: showcase-marquee 46s linear infinite;
  display: flex;
  gap: 1.1rem;
  width: max-content;
}

.showcase-code__column {
  display: flex;
  flex: 0 0 10.5rem;
  flex-direction: column;
  gap: 0.22rem;
  opacity: 0.62;
}

.showcase-code__column code {
  color: rgba(154, 149, 176, 0.55);
  font-family: 'DM Mono', monospace;
  font-size: 0.62rem;
  line-height: 1.55;
  min-height: 0.95rem;
  white-space: pre;
}

.showcase-code__column code.is-accent {
  color: rgba(72, 202, 228, 0.84);
  text-shadow: 0 0 18px rgba(72, 202, 228, 0.24);
}

.showcase-seam {
  background: linear-gradient(180deg, transparent, var(--cyan) 18%, var(--gold) 52%, var(--cyan) 82%, transparent);
  bottom: 0;
  box-shadow: 0 0 9px rgba(72, 202, 228, 0.9), 0 0 34px rgba(72, 202, 228, 0.45);
  left: 41%;
  position: absolute;
  top: 2.75rem;
  width: 1px;
  z-index: 6;
}

.showcase-seam::before {
  background: radial-gradient(circle, rgba(72, 202, 228, 0.25), transparent 68%);
  content: '';
  inset: 8% -3.5rem;
  position: absolute;
}

.showcase-seam__spark {
  background: var(--ivory);
  border-radius: 50%;
  box-shadow: 0 0 12px 3px rgba(72, 202, 228, 0.72);
  height: 3px;
  left: -1px;
  position: absolute;
  width: 3px;
}

.showcase-seam__spark--one { top: 23%; }
.showcase-seam__spark--two { top: 51%; }
.showcase-seam__spark--three { top: 77%; }

.showcase-products {
  align-items: center;
  display: flex;
  margin-left: 41%;
  min-height: 34rem;
  padding: 5rem 2rem 2.4rem;
  position: relative;
  z-index: 2;
}

.showcase-surfaces {
  display: grid;
  gap: clamp(0.8rem, 1.4vw, 1.3rem);
  grid-template-columns: repeat(3, minmax(0, 1fr));
  width: 100%;
}

.product-surface {
  background: rgba(5, 16, 29, 0.9);
  border: 1px solid rgba(72, 202, 228, 0.2);
  border-radius: 0.85rem;
  box-shadow: 0 20px 42px rgba(0, 0, 0, 0.45), 0 0 22px rgba(72, 202, 228, 0.07), inset 0 1px rgba(255, 255, 255, 0.05);
  min-width: 0;
  overflow: hidden;
  position: relative;
  top: -0.75rem;
  transition: border-color 220ms var(--ease-out), box-shadow 220ms var(--ease-out), transform 220ms var(--ease-out);
}

.product-surface:nth-child(2) {
  top: 0.85rem;
}

.product-surface:nth-child(3) {
  top: -0.25rem;
}

.product-surface:hover {
  border-color: rgba(212, 191, 148, 0.5);
  box-shadow: 0 28px 58px rgba(0, 0, 0, 0.52), 0 0 28px rgba(72, 202, 228, 0.12), inset 0 1px rgba(255, 255, 255, 0.07);
  transform: translateY(-6px);
}

.product-surface__visual,
.mission-feed {
  aspect-ratio: 16 / 10;
  background: #03101c;
  border-bottom: 1px solid rgba(72, 202, 228, 0.13);
  overflow: hidden;
  position: relative;
}

.product-surface__visual::after {
  background: linear-gradient(180deg, transparent 58%, rgba(3, 10, 20, 0.8));
  content: '';
  inset: 0;
  position: absolute;
}

.product-surface__visual img {
  height: 100%;
  object-fit: cover;
  object-position: top left;
  transition: transform 500ms var(--ease-out);
  width: 100%;
}

.product-surface:hover .product-surface__visual img {
  transform: scale(1.035);
}

.product-surface__signal {
  background: var(--cyan);
  border: 2px solid rgba(3, 10, 20, 0.8);
  border-radius: 50%;
  box-shadow: 0 0 12px rgba(72, 202, 228, 0.8);
  height: 0.55rem;
  position: absolute;
  right: 0.75rem;
  top: 0.75rem;
  width: 0.55rem;
  z-index: 2;
}

.product-surface__copy {
  min-height: 8.1rem;
  padding: 1rem 1rem 1.15rem;
}

.product-surface__kicker {
  color: var(--cyan);
  display: block;
  font-family: 'DM Mono', monospace;
  font-size: 0.56rem;
  letter-spacing: 0.14em;
  margin-bottom: 0.45rem;
  text-transform: uppercase;
}

.product-surface__copy h3 {
  color: var(--ivory);
  font-size: 0.9rem;
  font-weight: 700;
  letter-spacing: -0.015em;
}

.product-surface__copy p {
  font-size: 0.7rem;
  line-height: 1.55;
  margin-top: 0.45rem;
}

.mission-feed {
  padding: 0.85rem;
}

.mission-feed__header {
  align-items: center;
  color: rgba(178, 228, 239, 0.64);
  display: flex;
  font-family: 'DM Mono', monospace;
  font-size: 0.54rem;
  justify-content: space-between;
  letter-spacing: 0.12em;
  text-transform: uppercase;
}

.mission-feed__live {
  border: 1px solid rgba(72, 202, 228, 0.34);
  border-radius: 999px;
  color: var(--cyan);
  padding: 0.18rem 0.4rem;
}

.mission-feed__metric {
  align-items: baseline;
  display: flex;
  gap: 0.5rem;
  margin: 1rem 0 0.75rem;
}

.mission-feed__metric strong {
  color: var(--ivory);
  font-family: 'DM Mono', monospace;
  font-size: clamp(1.45rem, 2.5vw, 2rem);
  line-height: 1;
  text-shadow: 0 0 18px rgba(72, 202, 228, 0.35);
}

.mission-feed__metric span {
  color: var(--text-muted);
  font-size: 0.58rem;
}

.mission-feed ul {
  display: grid;
  gap: 0.42rem;
  margin: 0;
  padding: 0;
}

.mission-feed li {
  align-items: center;
  background: rgba(12, 31, 49, 0.74);
  border: 1px solid rgba(72, 202, 228, 0.11);
  border-radius: 0.42rem;
  color: rgba(242, 239, 232, 0.72);
  display: flex;
  font-size: 0.52rem;
  gap: 0.45rem;
  line-height: 1.25;
  min-height: 1.75rem;
  padding: 0.3rem 0.4rem;
}

.mission-feed__agent {
  align-items: center;
  background: rgba(242, 239, 232, 0.08);
  border: 1px solid rgba(242, 239, 232, 0.24);
  border-radius: 50%;
  color: var(--ivory);
  display: inline-flex;
  flex: 0 0 1.15rem;
  font-family: 'DM Mono', monospace;
  height: 1.15rem;
  justify-content: center;
}

.mission-feed__agent--gold {
  background: rgba(212, 191, 148, 0.13);
  border-color: rgba(212, 191, 148, 0.45);
  color: var(--gold);
}

.mission-feed__agent--blue {
  background: rgba(72, 202, 228, 0.12);
  border-color: rgba(72, 202, 228, 0.42);
  color: var(--cyan);
}

@keyframes showcase-marquee {
  from { transform: translateX(0); }
  to { transform: translateX(-50%); }
}

@media (max-width: 900px) {
  .showcase-stage {
    min-height: 0;
  }

  .showcase-code {
    height: 13rem;
    inset: auto;
    margin-top: 2.75rem;
    mask-image: linear-gradient(180deg, black 0%, black 60%, transparent 100%);
    padding-block-start: 1.25rem;
    position: relative;
  }

  .showcase-seam {
    background: linear-gradient(90deg, transparent, var(--cyan) 18%, var(--gold) 52%, var(--cyan) 82%, transparent);
    bottom: auto;
    height: 1px;
    left: 0;
    right: 0;
    top: 15.75rem;
    width: auto;
  }

  .showcase-seam::before {
    inset: -2.5rem 10%;
  }

  .showcase-seam__spark {
    bottom: -1px;
    left: auto;
    top: auto;
  }

  .showcase-seam__spark--one { left: 22%; }
  .showcase-seam__spark--two { left: 51%; }
  .showcase-seam__spark--three { left: 78%; }

  .showcase-products {
    margin-left: 0;
    min-height: 0;
    padding: 3rem 1rem 2rem;
  }

  .showcase-surfaces {
    display: flex;
    gap: 1rem;
    overflow-x: auto;
    padding: 0.75rem 0.25rem 1rem;
    scroll-padding-inline: 0.25rem;
    scroll-snap-type: x mandatory;
    scrollbar-color: rgba(72, 202, 228, 0.28) transparent;
  }

  .product-surface {
    flex: 0 0 min(78vw, 20rem);
    scroll-snap-align: center;
    top: 0 !important;
  }
}

@media (max-width: 520px) {
  .showcase-stage__topbar {
    font-size: 0.52rem;
    letter-spacing: 0.1em;
    padding-inline: 0.8rem;
  }

  .showcase-code {
    padding-inline-start: 0.85rem;
  }

  .showcase-code__track {
    gap: 0.7rem;
  }

  .showcase-code__column {
    flex-basis: 10rem;
  }

  .showcase-eyebrow {
    font-size: 0.62rem;
    letter-spacing: 0.12em;
  }

  .showcase-eyebrow::before,
  .showcase-eyebrow::after {
    width: 1rem;
  }
}

@media (prefers-reduced-motion: reduce) {
  .showcase-code__track {
    animation: none;
  }

  .product-surface,
  .product-surface:hover,
  .product-surface__visual img,
  .product-surface:hover .product-surface__visual img {
    transform: none;
    transition: none;
  }

  /* v-motion sets inline opacity/transform via JS; force the final state so
     the staggered reveal doesn't animate under reduced motion */
  .product-surface {
    opacity: 1 !important;
    transform: none !important;
  }
}
</style>
