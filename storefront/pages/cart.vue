<template>
  <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10">
    <h1 class="text-3xl font-bold text-gray-900 mb-8">Shopping Cart</h1>

    <!-- Loading -->
    <div v-if="cartStore.loading && !cartStore.cart" class="py-16 text-center">
      <div class="inline-block h-8 w-8 border-4 border-indigo-600 border-t-transparent rounded-full animate-spin" />
      <p class="mt-4 text-gray-500">Loading cart...</p>
    </div>

    <!-- Empty cart -->
    <div v-else-if="cartStore.isEmpty" class="text-center py-16">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="mx-auto h-16 w-16 text-gray-300"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="1.5"
          d="M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 100 4 2 2 0 000-4z"
        />
      </svg>
      <p class="mt-4 text-lg text-gray-500">Your cart is empty.</p>
      <NuxtLink
        to="/products"
        class="mt-6 inline-flex items-center px-5 py-2.5 bg-indigo-600 text-white text-sm font-medium rounded-md hover:bg-indigo-700 transition-colors"
      >
        Continue Shopping
      </NuxtLink>
    </div>

    <!-- Cart with items -->
    <div v-else class="grid grid-cols-1 lg:grid-cols-3 gap-10">
      <!-- Cart items -->
      <div class="lg:col-span-2">
        <div class="bg-white rounded-lg shadow-sm border border-gray-200 overflow-hidden">
          <table class="min-w-full divide-y divide-gray-200">
            <thead class="bg-gray-50">
              <tr>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Product
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Price
                </th>
                <th class="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Quantity
                </th>
                <th class="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Total
                </th>
                <th class="px-6 py-3" />
              </tr>
            </thead>
            <tbody class="bg-white divide-y divide-gray-200">
              <tr v-for="item in cartStore.items" :key="item.id">
                <!-- Product -->
                <td class="px-6 py-4">
                  <div class="flex items-center">
                    <div class="h-16 w-16 flex-shrink-0 bg-gray-100 rounded-md flex items-center justify-center">
                      <svg
                        xmlns="http://www.w3.org/2000/svg"
                        class="h-8 w-8 text-gray-300"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z" />
                      </svg>
                    </div>
                    <div class="ml-4">
                      <NuxtLink
                        :to="`/products/${item.product_slug}`"
                        class="text-sm font-medium text-gray-900 hover:text-indigo-600"
                      >
                        {{ item.product_name }}
                      </NuxtLink>
                      <p class="text-xs text-gray-500 mt-0.5">SKU: {{ item.variant_sku }}</p>
                    </div>
                  </div>
                </td>

                <!-- Price -->
                <td class="px-6 py-4 text-sm text-gray-900">
                  {{ formatPrice(item.variant_price) }}
                </td>

                <!-- Quantity controls -->
                <td class="px-6 py-4">
                  <div class="flex items-center border border-gray-300 rounded-md w-fit">
                    <button
                      class="px-2 py-1 text-gray-600 hover:text-gray-900 text-sm transition-colors"
                      :disabled="item.quantity <= 1 || cartStore.loading"
                      @click="updateQuantity(item.id, item.quantity - 1)"
                    >
                      -
                    </button>
                    <span class="px-3 py-1 text-sm text-gray-900 font-medium min-w-[2.5rem] text-center">
                      {{ item.quantity }}
                    </span>
                    <button
                      class="px-2 py-1 text-gray-600 hover:text-gray-900 text-sm transition-colors"
                      :disabled="cartStore.loading"
                      @click="updateQuantity(item.id, item.quantity + 1)"
                    >
                      +
                    </button>
                  </div>
                </td>

                <!-- Line total -->
                <td class="px-6 py-4 text-sm font-medium text-gray-900 text-right">
                  {{ formatPrice(lineTotal(item)) }}
                </td>

                <!-- Remove -->
                <td class="px-6 py-4 text-right">
                  <button
                    class="text-gray-400 hover:text-red-600 transition-colors"
                    :disabled="cartStore.loading"
                    title="Remove item"
                    @click="removeItem(item.id)"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                    </svg>
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Continue shopping link -->
        <div class="mt-4">
          <NuxtLink
            to="/products"
            class="text-sm text-indigo-600 hover:text-indigo-800 inline-flex items-center"
          >
            <svg xmlns="http://www.w3.org/2000/svg" class="mr-1 h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
            </svg>
            Continue Shopping
          </NuxtLink>
        </div>
      </div>

      <!-- Checkout sidebar -->
      <div class="lg:col-span-1">
        <div class="bg-white rounded-lg shadow-sm border border-gray-200 p-6 sticky top-6">
          <h2 class="text-lg font-semibold text-gray-900 mb-4">Order Summary</h2>

          <!-- Country selector -->
          <div class="mb-4">
            <label for="checkout-country" class="block text-sm font-medium text-gray-700 mb-1">
              Ship to
            </label>
            <select
              id="checkout-country"
              v-model="selectedCountry"
              class="block w-full rounded-md border-gray-300 shadow-sm text-sm focus:border-indigo-500 focus:ring-indigo-500"
              @change="recalculate"
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

          <!-- B2B VAT number -->
          <div class="mb-5">
            <label for="vat-number" class="block text-sm font-medium text-gray-700 mb-1">
              EU VAT Number
              <span class="font-normal text-gray-400">(optional, B2B)</span>
            </label>
            <div class="flex gap-2">
              <input
                id="vat-number"
                v-model="vatNumber"
                type="text"
                placeholder="e.g. DE123456789"
                class="block w-full rounded-md border-gray-300 shadow-sm text-sm focus:border-indigo-500 focus:ring-indigo-500"
              />
              <button
                :disabled="!vatNumber.trim() || validatingVat"
                class="px-3 py-2 text-xs font-medium bg-gray-100 border border-gray-300 rounded-md hover:bg-gray-200 disabled:opacity-40 disabled:cursor-not-allowed transition-colors whitespace-nowrap"
                @click="validateVatNumber"
              >
                {{ validatingVat ? '...' : 'Validate' }}
              </button>
            </div>
            <p v-if="vatValidation?.valid" class="mt-1 text-xs text-green-600">
              Valid: {{ vatValidation.company_name }}
            </p>
            <p v-else-if="vatValidation && !vatValidation.valid" class="mt-1 text-xs text-red-600">
              {{ vatValidation.message || 'Invalid VAT number' }}
            </p>
          </div>

          <!-- Totals -->
          <div class="border-t border-gray-200 pt-4 space-y-2">
            <template v-if="cartStore.calculation">
              <div class="flex justify-between text-sm text-gray-600">
                <span>Subtotal</span>
                <span>{{ formatPrice(cartStore.calculation.subtotal) }}</span>
              </div>
              <div class="flex justify-between text-sm text-gray-600">
                <span>
                  VAT
                  <template v-if="cartStore.calculation.reverse_charge">
                    (Reverse Charge)
                  </template>
                </span>
                <span>{{ formatPrice(cartStore.calculation.vat_total) }}</span>
              </div>
              <div
                v-if="cartStore.calculation.discount_amount && parseFloat(cartStore.calculation.discount_amount) > 0"
                class="flex justify-between text-sm text-green-600"
              >
                <span>Discount</span>
                <span>-{{ formatPrice(cartStore.calculation.discount_amount) }}</span>
              </div>
              <div class="flex justify-between text-sm text-gray-600">
                <span>Shipping</span>
                <span>{{ formatPrice(cartStore.calculation.shipping_fee) }}</span>
              </div>
              <div class="border-t border-gray-200 pt-2 flex justify-between text-base font-semibold text-gray-900">
                <span>Total</span>
                <span>{{ formatPrice(cartStore.calculation.total) }}</span>
              </div>
              <p
                v-if="cartStore.calculation.reverse_charge"
                class="text-xs text-gray-500 mt-2"
              >
                VAT reverse charge applies. You are liable for VAT in your country of registration.
              </p>
            </template>
            <template v-else>
              <div class="flex justify-between text-sm text-gray-600">
                <span>Subtotal</span>
                <span>{{ formatPrice(cartSubtotal) }}</span>
              </div>
              <p class="text-xs text-gray-400 mt-1">
                Select a country to see VAT and shipping costs.
              </p>
            </template>
          </div>

          <!-- Error display -->
          <p v-if="cartStore.error" class="mt-3 text-sm text-red-600">{{ cartStore.error }}</p>
          <p v-if="checkoutError" class="mt-3 text-sm text-red-600">{{ checkoutError }}</p>

          <!-- Checkout button -->
          <button
            :disabled="!selectedCountry || checkingOut || cartStore.isEmpty"
            class="mt-5 w-full px-6 py-3 bg-indigo-600 text-white font-medium rounded-md hover:bg-indigo-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            @click="proceedToCheckout"
          >
            {{ checkingOut ? 'Redirecting...' : 'Proceed to Checkout' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Country, VATValidation } from '~/types'

useHead({
  title: 'Cart - ForgeCommerce',
})

const cartStore = useCartStore()

const countries = ref<Country[]>([])
const selectedCountry = ref('')
const vatNumber = ref('')
const vatValidation = ref<VATValidation | null>(null)
const validatingVat = ref(false)
const checkingOut = ref(false)
const checkoutError = ref<string | null>(null)

function formatPrice(price: string): string {
  const num = parseFloat(price)
  return `€${num.toFixed(2)}`
}

function lineTotal(item: { variant_price: string; quantity: number }): string {
  const price = parseFloat(item.variant_price)
  return (price * item.quantity).toFixed(2)
}

const cartSubtotal = computed(() => {
  return cartStore.items
    .reduce((sum, item) => sum + parseFloat(item.variant_price) * item.quantity, 0)
    .toFixed(2)
})

async function updateQuantity(itemId: string, newQuantity: number) {
  if (newQuantity < 1) return
  await cartStore.updateItemQuantity(itemId, newQuantity)
  if (selectedCountry.value) {
    recalculate()
  }
}

async function removeItem(itemId: string) {
  await cartStore.removeItem(itemId)
  if (selectedCountry.value) {
    recalculate()
  }
}

async function recalculate() {
  if (!selectedCountry.value) return
  await cartStore.calculateTotals(
    selectedCountry.value,
    vatValidation.value?.valid ? vatNumber.value : undefined
  )
}

async function validateVatNumber() {
  if (!vatNumber.value.trim()) return
  validatingVat.value = true
  vatValidation.value = null
  try {
    const { post } = useApi()
    vatValidation.value = await post<VATValidation>(
      `/api/v1/cart/${cartStore.cartId}/vat-number`,
      { vat_number: vatNumber.value.trim().toUpperCase().replace(/\s/g, '') }
    )
    if (selectedCountry.value) {
      recalculate()
    }
  } catch (e: any) {
    vatValidation.value = {
      valid: false,
      vat_number: vatNumber.value,
      message: e.message || 'Validation failed',
    }
  } finally {
    validatingVat.value = false
  }
}

async function proceedToCheckout() {
  if (!selectedCountry.value || !cartStore.cartId) return

  checkingOut.value = true
  checkoutError.value = null

  try {
    const { post } = useApi()
    const res = await post<{ checkout_url: string }>('/api/v1/checkout', {
      cart_id: cartStore.cartId,
      country_code: selectedCountry.value,
      vat_number: vatValidation.value?.valid ? vatNumber.value.trim().toUpperCase().replace(/\s/g, '') : '',
    })

    // Redirect to Stripe Checkout.
    if (res.checkout_url) {
      window.location.href = res.checkout_url
    }
  } catch (e: any) {
    checkoutError.value = e.message || 'Failed to start checkout'
    checkingOut.value = false
  }
}

async function fetchCountries() {
  try {
    const { get } = useApi()
    countries.value = await get<Country[]>('/api/v1/countries')
  } catch {
    countries.value = []
  }
}

onMounted(() => {
  fetchCountries()
})
</script>
