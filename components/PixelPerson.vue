<script setup lang="ts">
// Pixel-art person rendered with a single box-shadow — no image assets.
// Pattern legend: H hair · S skin · E eyes · T shirt · P pants · B shoes · . empty
const props = withDefaults(
  defineProps<{ hair: string; shirt: string; scale?: number; animated?: boolean }>(),
  { scale: 5, animated: true }
)

const PATTERN = [
  '....HHHH....',
  '...HHHHHH...',
  '...HSSSSH...',
  '...SSSSSS...',
  '...SESSES...',
  '...SSSSSS...',
  '....SSSS....',
  '..TTTTTTTT..',
  '.TTTTTTTTTT.',
  '.STTTTTTTTS.',
  '.STTTTTTTTS.',
  '..TTTTTTTT..',
  '...PP..PP...',
  '...PP..PP...',
  '...BB..BB...'
]

const palette = computed<Record<string, string>>(() => ({
  H: props.hair,
  S: '#e8b98f',
  E: '#1b1b26',
  T: props.shirt,
  P: '#3a3a4d',
  B: '#14141f'
}))

const shadow = computed(() => {
  const s = props.scale
  const parts: string[] = []
  PATTERN.forEach((row, y) => {
    ;[...row].forEach((ch, x) => {
      const color = palette.value[ch]
      if (color) parts.push(`${x * s}px ${y * s}px 0 0 ${color}`)
    })
  })
  return parts.join(',')
})

const w = computed(() => PATTERN[0].length * props.scale)
const h = computed(() => PATTERN.length * props.scale)
</script>

<template>
  <div :class="animated ? 'px-bob' : ''" :style="{ width: w + 'px', height: h + 'px' }" aria-hidden="true">
    <div :style="{ width: scale + 'px', height: scale + 'px', boxShadow: shadow, marginInlineStart: '-' + scale + 'px', marginBlockStart: '-' + scale + 'px', transform: 'translate(' + scale + 'px,' + scale + 'px)' }" />
  </div>
</template>
