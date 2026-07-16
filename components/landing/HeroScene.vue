<script setup lang="ts">
import * as THREE from 'three'

// Refined data-orb: one sparse Fibonacci point sphere + two thin, slow
// counter-rotating rings (armillary-style). Champagne-gold + cyan dual
// accent. Init is deferred until the panel is on-screen and the browser is
// idle, so it never competes with LCP.
const mount = ref<HTMLDivElement | null>(null)
let cleanup: (() => void) | null = null
let io: IntersectionObserver | null = null

function initScene(host: HTMLDivElement) {
  const el = document.createElement('canvas')
  el.style.width = '100%'; el.style.height = '100%'; el.style.display = 'block'
  host.appendChild(el)
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

  const renderer = new THREE.WebGLRenderer({ canvas: el, alpha: true, antialias: true })
  renderer.setPixelRatio(Math.min(devicePixelRatio, 2))
  const scene = new THREE.Scene()
  scene.fog = new THREE.Fog(0x050a16, 3, 6.5)
  const cam = new THREE.PerspectiveCamera(38, 2, 0.1, 100)
  cam.position.z = 3.6

  // soft round sprite for points + core glow, shared
  const cv = document.createElement('canvas'); cv.width = cv.height = 32
  const cg = cv.getContext('2d')!
  const gr = cg.createRadialGradient(16, 16, 0, 16, 16, 16)
  gr.addColorStop(0, 'rgba(255,255,255,1)'); gr.addColorStop(0.5, 'rgba(255,255,255,.55)'); gr.addColorStop(1, 'rgba(255,255,255,0)')
  cg.fillStyle = gr; cg.fillRect(0, 0, 32, 32)
  const sprite = new THREE.CanvasTexture(cv)

  const root = new THREE.Group(); scene.add(root); root.rotation.x = 0.15
  const cyan = new THREE.Color('#48CAE4')
  const gold = new THREE.Color('#d4bf94')

  // one fine, evenly-distributed point sphere (Fibonacci lattice) — a jewel, not a scribble
  const COUNT = 620
  const positions = new Float32Array(COUNT * 3)
  const colors = new Float32Array(COUNT * 3)
  const phi = Math.PI * (3 - Math.sqrt(5))
  for (let i = 0; i < COUNT; i++) {
    const y = 1 - (i / (COUNT - 1)) * 2
    const r = Math.sqrt(Math.max(0, 1 - y * y))
    const theta = phi * i
    positions[i * 3] = Math.cos(theta) * r
    positions[i * 3 + 1] = y
    positions[i * 3 + 2] = Math.sin(theta) * r
    const c = i % 5 === 0 ? gold : cyan
    colors[i * 3] = c.r; colors[i * 3 + 1] = c.g; colors[i * 3 + 2] = c.b
  }
  const pGeo = new THREE.BufferGeometry()
  pGeo.setAttribute('position', new THREE.BufferAttribute(positions, 3))
  pGeo.setAttribute('color', new THREE.BufferAttribute(colors, 3))
  const points = new THREE.Points(pGeo, new THREE.PointsMaterial({
    size: 0.05, sizeAttenuation: true, map: sprite, vertexColors: true,
    transparent: true, opacity: 0.4, blending: THREE.AdditiveBlending, depthWrite: false,
  }))
  points.scale.setScalar(1.08)
  root.add(points)

  // two thin halo rings, gold + cyan, counter-rotating
  function ringGeometry(r: number, seg = 128) {
    const pts: THREE.Vector3[] = []
    for (let i = 0; i <= seg; i++) {
      const a = (i / seg) * Math.PI * 2
      pts.push(new THREE.Vector3(Math.cos(a) * r, 0, Math.sin(a) * r))
    }
    return new THREE.BufferGeometry().setFromPoints(pts)
  }
  const ringGeo = ringGeometry(1.34)
  const ringGold = new THREE.LineLoop(ringGeo, new THREE.LineBasicMaterial({ color: gold, transparent: true, opacity: 0.22, blending: THREE.AdditiveBlending, depthWrite: false }))
  ringGold.rotation.set(1.18, 0, 0.35)
  const ringCyan = new THREE.LineLoop(ringGeo, new THREE.LineBasicMaterial({ color: cyan, transparent: true, opacity: 0.18, blending: THREE.AdditiveBlending, depthWrite: false }))
  ringCyan.rotation.set(-1.02, 0, -0.5)
  root.add(ringGold, ringCyan)

  // faint core glow — warm-cool blend, small enough to read as a spark, not a marble
  const core = new THREE.Sprite(new THREE.SpriteMaterial({ map: sprite, color: new THREE.Color('#f3e9d2'), transparent: true, opacity: 0.24, blending: THREE.AdditiveBlending, depthWrite: false }))
  core.scale.set(0.16, 0.16, 1)
  root.add(core)

  function resize() {
    const w = host.clientWidth || 600, h = host.clientHeight || 300
    renderer.setSize(w, h, false); cam.aspect = w / h; cam.updateProjectionMatrix()
  }
  resize()

  let t = 0, raf = 0
  function tick() {
    resize(); t += 0.016
    root.rotation.y += 0.0011 // ~90s per revolution — deliberate, unhurried
    root.position.y = Math.sin(t * 0.18) * 0.05
    ringGold.rotation.y += 0.0009
    ringCyan.rotation.y -= 0.0007
    core.material.opacity = 0.2 + Math.sin(t * 0.6) * 0.05
    renderer.render(scene, cam)
    raf = requestAnimationFrame(tick)
  }
  if (reduce) renderer.render(scene, cam); else tick()

  cleanup = () => {
    cancelAnimationFrame(raf)
    renderer.dispose(); pGeo.dispose(); ringGeo.dispose(); sprite.dispose()
    el.remove()
  }
}

onMounted(() => {
  const host = mount.value
  if (!host) return
  const ric: (cb: () => void) => void = (window as any).requestIdleCallback || ((cb: () => void) => setTimeout(cb, 1))
  io = new IntersectionObserver((entries) => {
    if (entries[0]?.isIntersecting) {
      io?.disconnect(); io = null
      ric(() => initScene(host))
    }
  }, { rootMargin: '200px' })
  io.observe(host)
})
onBeforeUnmount(() => { io?.disconnect(); cleanup?.() })
</script>

<template>
  <div ref="mount" class="hero-scene" aria-hidden="true" />
</template>

<style scoped>
.hero-scene { width: 100%; height: 100%; }
</style>
