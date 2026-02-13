<template>
  <div>
    <!-- Hero Section -->
    <section class="bg-white">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-16 sm:py-24">
        <div class="text-center">
          <h1 class="text-4xl font-extrabold tracking-tight text-gray-900 sm:text-5xl md:text-6xl">
            Quality Crafted Goods
          </h1>
          <p class="mt-4 max-w-2xl mx-auto text-xl text-gray-500">
            Handmade products from EU manufacturers, shipped across Europe with full VAT compliance.
          </p>
          <div class="mt-8">
            <NuxtLink
              to="/products"
              class="inline-flex items-center px-6 py-3 border border-transparent text-base font-medium rounded-md text-white bg-indigo-600 hover:bg-indigo-700 transition-colors"
            >
              Browse All Products
            </NuxtLink>
          </div>
        </div>
      </div>
    </section>

    <!-- Featured Products -->
    <section class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
      <h2 class="text-2xl font-bold text-gray-900 mb-8">Featured Products</h2>

      <!-- Loading state -->
      <div v-if="loading" class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6">
        <div
          v-for="n in 8"
          :key="n"
          class="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden animate-pulse"
        >
          <div class="aspect-square bg-gray-200" />
          <div class="p-4 space-y-3">
            <div class="h-4 bg-gray-200 rounded w-3/4" />
            <div class="h-4 bg-gray-200 rounded w-1/2" />
          </div>
        </div>
      </div>

      <!-- Error state -->
      <div
        v-else-if="error"
        class="text-center py-12"
      >
        <p class="text-red-600 text-lg">{{ error }}</p>
        <button
          class="mt-4 text-indigo-600 hover:text-indigo-800 underline"
          @click="fetchProducts"
        >
          Try again
        </button>
      </div>

      <!-- Products grid -->
      <div
        v-else-if="products.length"
        class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-6"
      >
        <NuxtLink
          v-for="product in products"
          :key="product.id"
          :to="`/products/${product.slug}`"
          class="group bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden hover:shadow-md transition-shadow"
        >
          <!-- Image placeholder -->
          <div class="aspect-square bg-gray-100 flex items-center justify-center">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              class="h-16 w-16 text-gray-300 group-hover:text-gray-400 transition-colors"
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

          <!-- Product info -->
          <div class="p-4">
            <h3 class="text-sm font-medium text-gray-900 group-hover:text-indigo-600 transition-colors line-clamp-2">
              {{ product.name }}
            </h3>
            <p
              v-if="product.short_description"
              class="mt-1 text-sm text-gray-500 line-clamp-1"
            >
              {{ product.short_description }}
            </p>
            <div class="mt-2 flex items-center gap-2">
              <span class="text-lg font-semibold text-gray-900">
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
      </div>

      <!-- Empty state -->
      <div v-else class="text-center py-12">
        <p class="text-gray-500 text-lg">No products available yet.</p>
      </div>

      <!-- View all link -->
      <div v-if="products.length" class="mt-10 text-center">
        <NuxtLink
          to="/products"
          class="inline-flex items-center px-5 py-2.5 text-sm font-medium text-indigo-600 bg-indigo-50 rounded-md hover:bg-indigo-100 transition-colors"
        >
          View All Products
          <svg xmlns="http://www.w3.org/2000/svg" class="ml-2 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
          </svg>
        </NuxtLink>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import type { PaginatedResponse, ProductSummary } from '~/types'

useHead({
  title: 'ForgeCommerce - Quality Crafted Goods',
})

const products = ref<ProductSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

function formatPrice(price: string): string {
  const num = parseFloat(price)
  return `€${num.toFixed(2)}`
}

async function fetchProducts() {
  loading.value = true
  error.value = null
  try {
    const { get } = useApi()
    const res = await get<PaginatedResponse<ProductSummary>>(
      '/api/v1/products?limit=12'
    )
    products.value = res.data
  } catch (e: any) {
    error.value = e.message || 'Failed to load products'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchProducts()
})
</script>
