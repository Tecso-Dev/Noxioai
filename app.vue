<script setup lang="ts">
const head = useLocaleHead({ dir: true, lang: true, seo: true })
</script>

<template>
  <Html :lang="head.htmlAttrs?.lang" :dir="head.htmlAttrs?.dir">
    <Head>
      <template v-for="link in head.link" :key="link.hid">
        <Link :rel="link.rel" :href="link.href" :hreflang="link.hreflang" />
      </template>
      <template v-for="meta in head.meta" :key="meta.hid">
        <Meta :property="meta.property" :content="meta.content" />
      </template>
    </Head>
    <Body class="bg-night text-snow antialiased">
      <NuxtPage />
    </Body>
  </Html>
</template>
