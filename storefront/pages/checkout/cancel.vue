<template>
  <div class="max-w-2xl mx-auto px-4 sm:px-6 lg:px-8 py-16 text-center">
    <!-- Cancel icon -->
    <div class="mx-auto flex items-center justify-center h-20 w-20 rounded-full bg-yellow-100">
      <svg
        xmlns="http://www.w3.org/2000/svg"
        class="h-10 w-10 text-yellow-600"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          stroke-width="2"
          d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-2.5L13.732 4c-.77-.833-1.964-.833-2.732 0L4.082 16.5c-.77.833.192 2.5 1.732 2.5z"
        />
      </svg>
    </div>

    <h1 class="mt-6 text-3xl font-bold text-gray-900">Checkout Cancelled</h1>

    <p class="mt-4 text-lg text-gray-600">
      Your checkout was cancelled and no payment has been taken.
      Your cart items are still saved if you would like to try again.
    </p>

    <div class="mt-10 flex flex-col sm:flex-row items-center justify-center gap-4">
      <NuxtLink
        to="/cart"
        class="inline-flex items-center px-6 py-3 bg-indigo-600 text-white font-medium rounded-md hover:bg-indigo-700 transition-colors"
      >
        Return to Cart
      </NuxtLink>
      <NuxtLink
        to="/products"
        class="inline-flex items-center px-6 py-3 border border-gray-300 bg-white text-gray-700 font-medium rounded-md hover:bg-gray-50 transition-colors"
      >
        Continue Shopping
      </NuxtLink>
    </div>
  </div>
</template>

<script setup lang="ts">
useHead({
  title: 'Checkout Cancelled - ForgeCommerce',
})
</script>
