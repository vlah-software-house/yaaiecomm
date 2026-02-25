<script setup lang="ts">
import type { ProductSummary } from '~/types'

const props = defineProps<{
  product: ProductSummary
}>()

function formatPrice(price: string): string {
  const num = parseFloat(price)
  return `€${num.toFixed(2)}`
}
</script>

<template>
  <NuxtLink
    :to="`/products/${product.slug}`"
    class="group block rounded-lg border border-gray-200 bg-white transition-shadow hover:shadow-md"
  >
    <!-- Featured image or placeholder -->
    <div class="aspect-square w-full rounded-t-lg bg-gray-100 overflow-hidden">
      <img
        v-if="product.featured_image"
        :src="product.featured_image.url"
        :alt="product.featured_image.alt_text || product.name"
        class="h-full w-full object-cover transition-transform group-hover:scale-105"
        loading="lazy"
      >
      <div v-else class="h-full w-full flex items-center justify-center">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="h-16 w-16 text-gray-300"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="1"
            d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"
          />
        </svg>
      </div>
    </div>

    <div class="p-4">
      <h3 class="text-sm font-medium text-gray-900 group-hover:text-indigo-600 line-clamp-2">
        {{ product.name }}
      </h3>

      <p
        v-if="product.short_description"
        class="mt-1 text-xs text-gray-500 line-clamp-1"
      >
        {{ product.short_description }}
      </p>

      <div class="mt-2 flex items-center gap-2">
        <span class="text-base font-semibold text-gray-900">
          {{ formatPrice(product.base_price) }}
        </span>
        <span
          v-if="product.compare_at_price"
          class="text-sm text-gray-400 line-through"
        >
          {{ formatPrice(product.compare_at_price) }}
        </span>
      </div>
    </div>
  </NuxtLink>
</template>
