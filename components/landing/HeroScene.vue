<script setup lang="ts">
import * as THREE from 'three'

// JARVIS core: a filament data-sphere with orbiting agent orbs — the AI "team"
// as glowing data-entities. Canvas is created imperatively in onMounted so the
// element is always valid regardless of client-component remount timing.
const mount = ref<HTMLDivElement | null>(null)
let cleanup: (() => void) | null = null
const AGENT_HUES = [286, 194, 145, 344] // marketing / dev / support / social

onMounted(() => {
  const host = mount.value
  if (!host) return
  const el = document.createElement('canvas')
  el.style.width = '100%'; el.style.height = '100%'; el.style.display = 'block'
  host.appendChild(el)
  const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches

  const renderer = new THREE.WebGLRenderer({ canvas: el, alpha: true, antialias: true })
  renderer.setPixelRatio(Math.min(devicePixelRatio, 2))
  const scene = new THREE.Scene()
  scene.fog = new THREE.Fog(0x030b13, 3.2, 7)
  const cam = new THREE.PerspectiveCamera(40, 2, 0.1, 100)
  cam.position.z = 3.7

  const cv = document.createElement('canvas'); cv.width = cv.height = 64
  const cg = cv.getContext('2d')!
  const gr = cg.createRadialGradient(32, 32, 0, 32, 32, 32)
  gr.addColorStop(0, 'rgba(255,255,255,1)'); gr.addColorStop(0.4, 'rgba(255,255,255,.7)'); gr.addColorStop(1, 'rgba(255,255,255,0)')
  cg.fillStyle = gr; cg.fillRect(0, 0, 64, 64)
  const sprite = new THREE.CanvasTexture(cv)

  const root = new THREE.Group(); scene.add(root); root.rotation.x = 0.12
  const cyan = new THREE.Color().setHSL(194 / 360, 0.9, 0.62)
  const families: THREE.Group[] = []
  function arcFamily(tx: number, tz: number, count: number, r0: number, r1: number, spin: number) {
    const fam = new THREE.Group(); fam.rotation.x = tx; fam.rotation.z = tz
    const seg: THREE.Vector3[] = []
    for (let i = 0; i < count; i++) {
      const r = r0 + Math.random() * (r1 - r0)
      const a0 = Math.random() * Math.PI * 2, span = 0.4 + Math.random() * 2.2
      const q = new THREE.Quaternion().setFromEuler(new THREE.Euler((Math.random() - 0.5) * 0.5, 0, (Math.random() - 0.5) * 0.5))
      let prev: THREE.Vector3 | null = null; const n = Math.max(6, (span * 12) | 0)
      for (let j = 0; j <= n; j++) {
        const a = a0 + span * j / n
        const p = new THREE.Vector3(Math.cos(a) * r, (Math.random() - 0.5) * 0.01, Math.sin(a) * r).applyQuaternion(q)
        if (prev) seg.push(prev, p); prev = p
      }
    }
    const g = new THREE.BufferGeometry().setFromPoints(seg)
    fam.add(new THREE.LineSegments(g, new THREE.LineBasicMaterial({ color: cyan, transparent: true, opacity: 0.85, blending: THREE.AdditiveBlending, depthWrite: false })))
    const halo = new THREE.LineSegments(g, new THREE.LineBasicMaterial({ color: cyan, transparent: true, opacity: 0.25, blending: THREE.AdditiveBlending, depthWrite: false }))
    halo.scale.setScalar(1.012); fam.add(halo)
    fam.userData.spin = spin; root.add(fam); families.push(fam)
  }
  arcFamily(0, 0, 110, 0.35, 1.05, 0.0016)
  arcFamily(0.5, 0.2, 85, 0.4, 1.0, -0.0012)
  arcFamily(-0.4, 0.55, 70, 0.5, 1.05, 0.0009)

  const heart = new THREE.Sprite(new THREE.SpriteMaterial({ map: sprite, color: new THREE.Color().setHSL(194 / 360, 0.4, 0.98), transparent: true, opacity: 1, blending: THREE.AdditiveBlending, depthWrite: false }))
  heart.scale.set(1.4, 1.4, 1); root.add(heart)

  type Orb = { grp: THREE.Group; dir: THREE.Vector3; axis: THREE.Vector3; ang: number; v: number; seed: number; tether: THREE.BufferGeometry }
  const orbs: Orb[] = []
  AGENT_HUES.forEach((hue, i) => {
    const col = new THREE.Color().setHSL(hue / 360, 0.85, 0.64)
    const g = new THREE.Group()
    const seg: THREE.Vector3[] = []
    for (let k = 0; k < 14; k++) {
      const r = 0.12 + Math.random() * 0.12, a0 = Math.random() * Math.PI * 2, span = 0.6 + Math.random() * 2
      const q = new THREE.Quaternion().setFromEuler(new THREE.Euler(Math.random() - 0.5, 0, Math.random() - 0.5))
      let prev: THREE.Vector3 | null = null
      for (let j = 0; j <= 8; j++) { const a = a0 + span * j / 8; const p = new THREE.Vector3(Math.cos(a) * r, 0, Math.sin(a) * r).applyQuaternion(q); if (prev) seg.push(prev, p); prev = p }
    }
    g.add(new THREE.LineSegments(new THREE.BufferGeometry().setFromPoints(seg), new THREE.LineBasicMaterial({ color: col, transparent: true, opacity: 0.85, blending: THREE.AdditiveBlending, depthWrite: false })))
    const glow = new THREE.Sprite(new THREE.SpriteMaterial({ map: sprite, color: col, transparent: true, opacity: 0.6, blending: THREE.AdditiveBlending, depthWrite: false }))
    glow.scale.set(0.55, 0.55, 1); g.add(glow)
    root.add(g)
    const u = Math.random() * 2 - 1, aa = (i / AGENT_HUES.length) * Math.PI * 2, s = Math.sqrt(1 - u * u)
    const fg = new THREE.BufferGeometry(); fg.setAttribute('position', new THREE.BufferAttribute(new Float32Array(6), 3))
    root.add(new THREE.Line(fg, new THREE.LineBasicMaterial({ color: col, transparent: true, opacity: 0.2, blending: THREE.AdditiveBlending, depthWrite: false })))
    orbs.push({ grp: g, dir: new THREE.Vector3(s * Math.cos(aa), u * 0.5, s * Math.sin(aa)), axis: new THREE.Vector3(0, 1, 0.3).normalize(), ang: aa, v: 0.0016 + i * 0.0004, seed: Math.random() * 7, tether: fg })
  })

  let px = 0, py = 0
  const onMove = (e: MouseEvent) => { px = e.clientX / innerWidth - 0.5; py = e.clientY / innerHeight - 0.5 }
  addEventListener('mousemove', onMove)

  function resize() {
    const w = host!.clientWidth || 600, h = host!.clientHeight || 300
    renderer.setSize(w, h, false); cam.aspect = w / h; cam.updateProjectionMatrix()
  }
  resize()

  const v = new THREE.Vector3()
  let t = 0, raf = 0
  function tick() {
    resize(); t += 0.016
    root.rotation.y += 0.0016
    root.position.x += (px * 0.4 - root.position.x) * 0.04
    root.position.y += ((Math.sin(t * 0.5) * 0.08 - py * 0.25) - root.position.y) * 0.05
    families.forEach(f => { f.rotation.y += f.userData.spin })
    heart.material.opacity = 0.8 + Math.sin(t * 1.8) * 0.15
    const hs = 1.3 + Math.sin(t * 1.8) * 0.1; heart.scale.set(hs, hs, 1)
    orbs.forEach((o) => {
      o.ang += o.v
      v.copy(o.dir).applyAxisAngle(o.axis, o.ang)
      o.grp.position.copy(v).multiplyScalar(1.9 + Math.sin(t * 1.1 + o.seed) * 0.06)
      o.grp.rotation.y += 0.01
      const pa = o.tether.attributes.position as THREE.BufferAttribute
      pa.setXYZ(0, 0, 0, 0); pa.setXYZ(1, o.grp.position.x, o.grp.position.y, o.grp.position.z); pa.needsUpdate = true
    })
    renderer.render(scene, cam)
    raf = requestAnimationFrame(tick)
  }
  if (reduce) renderer.render(scene, cam); else tick()

  cleanup = () => { cancelAnimationFrame(raf); removeEventListener('mousemove', onMove); renderer.dispose(); el.remove() }
})
onBeforeUnmount(() => cleanup?.())
</script>

<template>
  <div ref="mount" class="hero-scene" aria-hidden="true" />
</template>

<style scoped>
.hero-scene { width: 100%; height: 100%; }
</style>
