<template>
  <div class="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 py-16 text-center">
    <!-- Success icon -->
    <div class="mx-auto flex items-center justify-center h-20 w-20 rounded-full bg-green-100">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-10 w-10 text-green-600"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M5 13l4 4L19 7"
        />
      </svg>
    </div>

    <h1 class="mt-6 text-3xl font-bold text-gray-900">Thank You for Your Order!</h1>

    <p class="mt-4 text-lg text-gray-600">
      Your payment was processed successfully. We have received your order and will begin
      preparing it for shipment.
    </p>

    <div v-if="sessionId" class="mt-6 p-4 bg-gray-50 rounded-lg inline-block">
      <p class="text-sm text-gray-500">
        Session reference:
        <span class="font-mono text-gray-700">{{ sessionId }}</span>
      </p>
    </div>

    <p class="mt-6 text-gray-500">
      A confirmation email will be sent to you shortly with your order details and tracking
      information once your order ships.
    </p>

    <div class="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
      <NuxtLink
        to="/products"
        class="inline-flex items-center px-6 py-3 bg-indigo-600 text-white font-medium rounded-md hover:bg-indigo-700 transition-colors"
      >
        Continue Shopping
      </NuxtLink>
      <NuxtLink
        to="/"
        class="inline-flex items-center px-6 py-3 border border-gray-300 bg-white text-gray-700 font-medium rounded-md hover:bg-gray-50 transition-colors"
      >
        Back to Home
      </NuxtLink>
    </div>
  </div>
</template>

<script setup lang="ts">
useHead({
  title: 'Order Confirmed - ForgeCommerce',
})

const route = useRoute()
const sessionId = computed(() => route.query.session_id as string | undefined)

const cartStore = useCartStore()

onMounted(() => {
  // Clear the cart after a successful checkout.
  if (import.meta.client) {
    localStorage.removeItem('cart_id')
  }
  cartStore.$reset()
})
</script>
