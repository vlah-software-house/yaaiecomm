<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
    <!-- Loading state -->
    <div v-if="loading" class="animate-pulse">
      <div class="grid grid-cols-1 md:grid-cols-2 gap-10">
        <div class="aspect-square bg-gray-200 rounded-lg" />
        <div class="space-y-4">
          <div class="h-8 bg-gray-200 rounded w-3/4" />
          <div class="h-6 bg-gray-200 rounded w-1/3" />
          <div class="h-20 bg-gray-200 rounded w-full" />
          <div class="h-10 bg-gray-200 rounded w-1/2" />
        </div>
      </div>
    </div>

    <!-- Error state -->
    <div v-else-if="error" class="text-center py-16">
      <p class="text-red-600 text-lg">{{ error }}</p>
      <NuxtLink
        to="/products"
        class="mt-4 inline-block text-indigo-600 hover:text-indigo-800 underline"
      >
        Back to products
      </NuxtLink>
    </div>

    <!-- Product detail -->
    <template v-else-if="product">
      <!-- Breadcrumb -->
      <nav class="mb-6 text-sm text-gray-500">
        <NuxtLink to="/" class="hover:text-gray-700">Home</NuxtLink>
        <span class="mx-2">/</span>
        <NuxtLink to="/products" class="hover:text-gray-700">Products</NuxtLink>
        <span class="mx-2">/</span>
        <span class="text-gray-900">{{ product.name }}</span>
      </nav>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-10">
        <!-- Image gallery -->
        <div>
          <!-- Main image -->
          <div class="aspect-square bg-gray-100 rounded-lg overflow-hidden border border-gray-200">
            <img
              v-if="activeImage"
              :src="activeImage.url"
              :alt="activeImage.alt_text || product.name"
              class="h-full w-full object-contain"
            >
            <div v-else class="h-full w-full flex items-center justify-center">
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-24 w-24 text-gray-300"
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

          <!-- Thumbnail strip -->
          <div
            v-if="displayImages.length > 1"
            class="mt-3 flex gap-2 overflow-x-auto"
          >
            <button
              v-for="img in displayImages"
              :key="img.id"
              :class="[
                'flex-shrink-0 w-16 h-16 rounded-md overflow-hidden border-2 transition-all',
                activeImage?.id === img.id
                  ? 'border-indigo-600 ring-1 ring-indigo-600'
                  : 'border-gray-200 hover:border-gray-400',
              ]"
              @click="activeImageId = img.id"
            >
              <img
                :src="img.url"
                :alt="img.alt_text || product.name"
                class="h-full w-full object-cover"
                loading="lazy"
              >
            </button>
          </div>
        </div>

        <!-- Product info -->
        <div>
          <h1 class="text-3xl font-bold text-gray-900">{{ product.name }}</h1>

          <!-- Price -->
          <div class="mt-4 flex items-center gap-3">
            <span class="text-3xl font-bold text-gray-900">
              {{ formatPrice(effectivePrice) }}
            </span>
            <span
              v-if="effectiveCompareAtPrice"
              class="text-lg text-gray-400 line-through"
            >
              {{ formatPrice(effectiveCompareAtPrice) }}
            </span>
          </div>

          <!-- Short description -->
          <p v-if="product.short_description" class="mt-4 text-gray-600 leading-relaxed">
            {{ product.short_description }}
          </p>

          <!-- Variant Picker -->
          <div v-if="product.has_variants && product.attributes.length" class="mt-6 space-y-5">
            <div
              v-for="attribute in product.attributes"
              :key="attribute.id"
            >
              <label class="block text-sm font-medium text-gray-900 mb-2">
                {{ attribute.display_name }}
              </label>

              <!-- Color swatch -->
              <div v-if="attribute.attribute_type === 'color_swatch'" class="flex flex-wrap gap-2">
                <button
                  v-for="option in attribute.options"
                  :key="option.id"
                  :title="option.display_value"
                  :class="[
                    'w-9 h-9 rounded-full border-2 transition-all',
                    selectedOptions[attribute.name] === option.value
                      ? 'border-indigo-600 ring-2 ring-indigo-600 ring-offset-1'
                      : 'border-gray-300 hover:border-gray-400',
                  ]"
                  :style="{ backgroundColor: option.color_hex || '#ccc' }"
                  @click="selectOption(attribute.name, option.value)"
                />
              </div>

              <!-- Button group (default) -->
              <div v-else class="flex flex-wrap gap-2">
                <button
                  v-for="option in attribute.options"
                  :key="option.id"
                  :class="[
                    'px-4 py-2 text-sm font-medium rounded-md border transition-colors',
                    selectedOptions[attribute.name] === option.value
                      ? 'bg-indigo-600 border-indigo-600 text-white'
                      : 'bg-white border-gray-300 text-gray-700 hover:border-gray-400',
                  ]"
                  @click="selectOption(attribute.name, option.value)"
                >
                  {{ option.display_value }}
                </button>
              </div>
            </div>
          </div>

          <!-- Selected variant info -->
          <div v-if="selectedVariant" class="mt-4">
            <p class="text-sm text-gray-500">
              SKU: {{ selectedVariant.sku }}
            </p>
            <p
              :class="[
                'text-sm mt-1',
                selectedVariant.stock_quantity > 0 ? 'text-green-600' : 'text-red-600',
              ]"
            >
              {{ selectedVariant.stock_quantity > 0 ? `${selectedVariant.stock_quantity} in stock` : 'Out of stock' }}
            </p>
          </div>

          <!-- Quantity and Add to Cart -->
          <div class="mt-6 flex items-center gap-4">
            <div class="flex items-center border border-gray-300 rounded-md">
              <button
                class="px-3 py-2 text-gray-600 hover:text-gray-900 transition-colors"
                :disabled="quantity <= 1"
                @click="quantity = Math.max(1, quantity - 1)"
              >
                -
              </button>
              <span class="px-4 py-2 text-gray-900 font-medium min-w-[3rem] text-center">
                {{ quantity }}
              </span>
              <button
                class="px-3 py-2 text-gray-600 hover:text-gray-900 transition-colors"
                @click="quantity++"
              >
                +
              </button>
            </div>

            <button
              :disabled="addingToCart || !canAddToCart"
              class="flex-1 px-6 py-3 bg-indigo-600 text-white font-medium rounded-md hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              @click="handleAddToCart"
            >
              {{ addingToCart ? 'Adding...' : 'Add to Cart' }}
            </button>
          </div>

          <p v-if="cartError" class="mt-2 text-sm text-red-600">{{ cartError }}</p>
          <p v-if="addedToCart" class="mt-2 text-sm text-green-600">Added to cart successfully.</p>

          <!-- VAT / Country Section -->
          <div class="mt-8 border-t border-gray-200 pt-6">
            <h3 class="text-sm font-medium text-gray-900 mb-3">Shipping & VAT</h3>
            <!-- CountrySelector placeholder -->
            <div class="flex items-center gap-3">
              <label for="country-select" class="text-sm text-gray-600">Ship to:</label>
              <select
                id="country-select"
                v-model="selectedCountry"
                class="block w-48 rounded-md border-gray-300 shadow-sm text-sm focus:border-indigo-500 focus:ring-indigo-500"
                @change="onCountryChange"
              >
                <option value="">Select country</option>
                <option
                  v-for="country in countries"
                  :key="country.country_code"
                  :value="country.country_code"
                >
                  {{ country.name }}
                </option>
              </select>
            </div>
            <!-- VATDisplay placeholder -->
            <div v-if="selectedCountry" class="mt-3 p-3 bg-gray-50 rounded-md">
              <p class="text-sm text-gray-600">
                VAT will be calculated at checkout for
                <span class="font-medium">{{ countries.find(c => c.country_code === selectedCountry)?.name }}</span>.
              </p>
            </div>
          </div>
        </div>
      </div>

      <!-- Full description -->
      <div v-if="product.description" class="mt-12 border-t border-gray-200 pt-8">
        <h2 class="text-xl font-bold text-gray-900 mb-4">Description</h2>
        <div class="prose prose-gray max-w-none text-gray-600 leading-relaxed whitespace-pre-line">
          {{ product.description }}
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { ProductDetail, ProductVariant, ProductImage, Country } from '~/types'

const route = useRoute()
const slug = route.params.slug as string

const product = ref<ProductDetail | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)

const selectedOptions = ref<Record<string, string>>({})
const quantity = ref(1)
const addingToCart = ref(false)
const addedToCart = ref(false)
const cartError = ref<string | null>(null)
const activeImageId = ref<string | null>(null)

const countries = ref<Country[]>([])
const selectedCountry = ref('')

const cartStore = useCartStore()

useHead({
  title: computed(() =>
    product.value ? `${product.value.name} - ForgeCommerce` : 'Product - ForgeCommerce'
  ),
})

function formatPrice(price: string): string {
  const num = parseFloat(price)
  return `€${num.toFixed(2)}`
}

// Resolve images for the current view: variant-specific images take priority,
// falling back to product-level images.
const displayImages = computed<ProductImage[]>(() => {
  if (!product.value) return []

  // If a variant is selected and it has its own images, show those.
  if (selectedVariant.value && selectedVariant.value.images.length > 0) {
    return selectedVariant.value.images
  }

  // Otherwise show all product images (which includes product-level images).
  return product.value.images || []
})

// The currently active (displayed) image.
const activeImage = computed<ProductImage | null>(() => {
  const images = displayImages.value
  if (!images.length) return null

  // If user clicked a specific thumbnail, show that.
  if (activeImageId.value) {
    const found = images.find(img => img.id === activeImageId.value)
    if (found) return found
  }

  // Default: show the primary (featured) image, or the first one.
  return images.find(img => img.is_primary) || images[0]
})

// When the variant changes, reset the active image selection so the gallery
// switches to the new variant's images (or falls back to product-level).
watch(() => selectedVariant.value?.id, () => {
  activeImageId.value = null
})

const selectedVariant = computed<ProductVariant | null>(() => {
  if (!product.value) return null

  // For simple products (no variants), return the first variant.
  if (!product.value.has_variants && product.value.variants.length) {
    return product.value.variants[0]
  }

  // Match variant by selected options.
  const selected = selectedOptions.value
  const selectedKeys = Object.keys(selected)

  if (!selectedKeys.length) return null

  return product.value.variants.find((variant) => {
    if (!variant.is_active) return false
    return variant.options.every(
      (opt) => selected[opt.attribute_name] === opt.option_value
    )
  }) || null
})

const effectivePrice = computed(() => {
  if (selectedVariant.value?.price) {
    return selectedVariant.value.price
  }
  return product.value?.base_price || '0'
})

const effectiveCompareAtPrice = computed(() => {
  if (selectedVariant.value?.compare_at_price) {
    return selectedVariant.value.compare_at_price
  }
  return product.value?.compare_at_price || null
})

const canAddToCart = computed(() => {
  if (!product.value) return false

  // Simple product: just check stock on first variant.
  if (!product.value.has_variants) {
    const v = product.value.variants[0]
    return v && v.is_active && v.stock_quantity > 0
  }

  // Product with variants: require all options selected and variant in stock.
  if (!selectedVariant.value) return false
  return selectedVariant.value.stock_quantity > 0
})

function selectOption(attributeName: string, optionValue: string) {
  selectedOptions.value = {
    ...selectedOptions.value,
    [attributeName]: optionValue,
  }
  addedToCart.value = false
}

async function handleAddToCart() {
  const variantId = selectedVariant.value?.id
  if (!variantId) return

  addingToCart.value = true
  cartError.value = null
  addedToCart.value = false

  try {
    await cartStore.addItem(variantId, quantity.value)
    cartStore.persistCartId()
    addedToCart.value = true
  } catch (e: any) {
    cartError.value = e.message || 'Failed to add item to cart'
  } finally {
    addingToCart.value = false
  }
}

function onCountryChange() {
  // Country change handler - VAT recalculation would happen here in
  // a full implementation, or at checkout via the calculate endpoint.
}

async function fetchProduct() {
  loading.value = true
  error.value = null
  try {
    const { get } = useApi()
    product.value = await get<ProductDetail>(`/api/v1/products/${slug}`)

    // Pre-select first option for each attribute.
    if (product.value.has_variants && product.value.attributes.length) {
      const defaults: Record<string, string> = {}
      for (const attr of product.value.attributes) {
        if (attr.options.length) {
          defaults[attr.name] = attr.options[0].value
        }
      }
      selectedOptions.value = defaults
    }
  } catch (e: any) {
    error.value = e.message || 'Product not found'
  } finally {
    loading.value = false
  }
}

async function fetchCountries() {
  try {
    const { get } = useApi()
    countries.value = await get<Country[]>('/api/v1/countries')
  } catch {
    // Countries are optional for browsing; fail silently.
    countries.value = []
  }
}

onMounted(() => {
  fetchProduct()
  fetchCountries()
})
</script>
