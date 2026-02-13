<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
    <!-- Page header -->
    <div class="mb-8">
      <h1 class="text-3xl font-bold text-gray-900">Products</h1>
      <p class="mt-2 text-gray-500">Browse our full range of handcrafted products.</p>
    </div>

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
    <div v-else-if="error" class="text-center py-16">
      <p class="text-red-600 text-lg">{{ error }}</p>
      <button
        class="mt-4 text-indigo-600 hover:text-indigo-800 underline"
        @click="fetchProducts"
      >
        Try again
      </button>
    </div>

    <!-- Products grid -->
    <template v-else>
      <div
        v-if="products.length"
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
              class="mt-1 text-sm text-gray-500 line-clamp-2"
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
      <div v-else class="text-center py-16">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          class="mx-auto h-12 w-12 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
        </svg>
        <p class="mt-4 text-gray-500 text-lg">No products found.</p>
      </div>

      <!-- Pagination -->
      <nav
        v-if="totalPages > 1"
        class="mt-10 flex items-center justify-center gap-2"
        aria-label="Product pagination"
      >
        <button
          :disabled="currentPage <= 1"
          class="px-3 py-2 text-sm font-medium rounded-md border border-gray-300 bg-white text-gray-700 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          @click="goToPage(currentPage - 1)"
        >
          Previous
        </button>

        <template v-for="page in paginationRange" :key="page">
          <span v-if="page === '...'" class="px-2 text-gray-400">...</span>
          <button
            v-else
            :class="[
              'px-3 py-2 text-sm font-medium rounded-md border transition-colors',
              page === currentPage
                ? 'bg-indigo-600 border-indigo-600 text-white'
                : 'bg-white border-gray-300 text-gray-700 hover:bg-gray-50',
            ]"
            @click="goToPage(page as number)"
          >
            {{ page }}
          </button>
        </template>

        <button
          :disabled="currentPage >= totalPages"
          class="px-3 py-2 text-sm font-medium rounded-md border border-gray-300 bg-white text-gray-700 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          @click="goToPage(currentPage + 1)"
        >
          Next
        </button>
      </nav>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { PaginatedResponse, ProductSummary } from '~/types'

useHead({
  title: 'Products - ForgeCommerce',
})

const route = useRoute()
const router = useRouter()

const products = ref<ProductSummary[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const currentPage = ref(1)
const totalPages = ref(1)
const totalCount = ref(0)
const limit = 20

function formatPrice(price: string): string {
  const num = parseFloat(price)
  return `€${num.toFixed(2)}`
}

const paginationRange = computed(() => {
  const range: (number | string)[] = []
  const total = totalPages.value
  const current = currentPage.value

  if (total <= 7) {
    for (let i = 1; i <= total; i++) range.push(i)
    return range
  }

  range.push(1)

  if (current > 3) {
    range.push('...')
  }

  const start = Math.max(2, current - 1)
  const end = Math.min(total - 1, current + 1)

  for (let i = start; i <= end; i++) {
    range.push(i)
  }

  if (current < total - 2) {
    range.push('...')
  }

  range.push(total)

  return range
})

async function fetchProducts() {
  loading.value = true
  error.value = null
  try {
    const { get } = useApi()
    const res = await get<PaginatedResponse<ProductSummary>>(
      `/api/v1/products?page=${currentPage.value}&limit=${limit}`
    )
    products.value = res.data
    totalPages.value = res.total_pages
    totalCount.value = res.total
  } catch (e: any) {
    error.value = e.message || 'Failed to load products'
  } finally {
    loading.value = false
  }
}

function goToPage(page: number) {
  if (page < 1 || page > totalPages.value) return
  currentPage.value = page
  router.push({ query: { ...route.query, page: String(page) } })
  fetchProducts()
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

onMounted(() => {
  const pageParam = route.query.page
  if (pageParam) {
    const parsed = parseInt(pageParam as string, 10)
    if (!isNaN(parsed) && parsed > 0) {
      currentPage.value = parsed
    }
  }
  fetchProducts()
})
</script>
