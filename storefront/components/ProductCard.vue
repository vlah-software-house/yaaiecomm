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
    <!-- Image placeholder -->
    <div class="aspect-square w-full rounded-t-lg bg-gray-200" />

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
