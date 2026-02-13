export default defineNuxtPlugin(() => {
  useHead({
    titleTemplate: (title) => title ? `${title} - ForgeCommerce` : 'ForgeCommerce',
    meta: [
      { name: 'description', content: 'EU-first e-commerce platform with full VAT compliance.' },
      { property: 'og:site_name', content: 'ForgeCommerce' },
      { property: 'og:type', content: 'website' },
      { name: 'theme-color', content: '#0284c7' },
    ],
    link: [
      { rel: 'icon', type: 'image/svg+xml', href: '/favicon.svg' },
    ],
    htmlAttrs: {
      lang: 'en',
    },
  })
})
