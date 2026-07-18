<script setup lang="ts">
const head = useLocaleHead({ dir: true, lang: true, seo: true })

const organizationSchema = {
  '@context': 'https://schema.org',
  '@type': 'Organization',
  '@id': 'https://noxioai.com/#organization',
  name: 'NOXIOAI',
  url: 'https://noxioai.com/',
  logo: 'https://noxioai.com/brand/noxioai-logo.png',
  description: 'AI employees that work while you sleep',
  sameAs: [],
  contactPoint: {
    '@type': 'ContactPoint',
    contactType: 'customer support',
    email: 'hi@noxioai.com'
  }
}

const websiteSchema = {
  '@context': 'https://schema.org',
  '@type': 'WebSite',
  '@id': 'https://noxioai.com/#website',
  name: 'NOXIOAI',
  url: 'https://noxioai.com/',
  publisher: {
    '@id': 'https://noxioai.com/#organization'
  },
  potentialAction: {
    '@type': 'SearchAction',
    target: {
      '@type': 'EntryPoint',
      urlTemplate: 'https://noxioai.com/?q={search_term_string}'
    },
    'query-input': 'required name=search_term_string'
  }
}

useHead({
  script: [
    {
      key: 'schema-org-organization',
      id: 'schema-org-organization',
      type: 'application/ld+json',
      innerHTML: JSON.stringify(organizationSchema)
    },
    {
      key: 'schema-org-website',
      id: 'schema-org-website',
      type: 'application/ld+json',
      innerHTML: JSON.stringify(websiteSchema)
    }
  ]
})
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
    <Body class="noxio-shell bg-night text-snow antialiased">
      <NuxtPage />
    </Body>
  </Html>
</template>
