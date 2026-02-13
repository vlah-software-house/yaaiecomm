<template>
  <div class="min-h-screen flex flex-col bg-gray-50">
    <!-- Header -->
    <header class="bg-white border-b border-gray-200">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center justify-between h-16">
          <!-- Logo -->
          <NuxtLink to="/" class="text-xl font-bold text-gray-900">
            ForgeCommerce
          </NuxtLink>

          <!-- Navigation -->
          <nav class="hidden md:flex items-center space-x-8">
            <NuxtLink to="/" class="text-gray-600 hover:text-gray-900">
              Home
            </NuxtLink>
            <NuxtLink to="/products" class="text-gray-600 hover:text-gray-900">
              Products
            </NuxtLink>
          </nav>

          <!-- Right side -->
          <div class="flex items-center space-x-4">
            <!-- Cart -->
            <NuxtLink
              to="/cart"
              class="relative text-gray-600 hover:text-gray-900"
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                class="h-6 w-6"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M3 3h2l.4 2M7 13h10l4-8H5.4M7 13L5.4 5M7 13l-2.293 2.293c-.63.63-.184 1.707.707 1.707H17m0 0a2 2 0 100 4 2 2 0 000-4zm-8 2a2 2 0 100 4 2 2 0 000-4z"
                />
              </svg>
              <span
                v-if="cartStore.itemCount > 0"
                class="absolute -top-2 -right-2 bg-primary-600 text-white text-xs rounded-full h-5 w-5 flex items-center justify-center"
              >
                {{ cartStore.itemCount }}
              </span>
            </NuxtLink>

            <!-- Auth -->
            <NuxtLink
              v-if="!authStore.isAuthenticated"
              to="/login"
              class="text-gray-600 hover:text-gray-900 text-sm"
            >
              Sign In
            </NuxtLink>
            <div v-else class="text-sm text-gray-600">
              {{ authStore.displayName }}
            </div>
          </div>
        </div>
      </div>
    </header>

    <!-- Main content -->
    <main class="flex-1">
      <slot />
    </main>

    <!-- Footer -->
    <footer class="bg-white border-t border-gray-200 mt-auto">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <p class="text-center text-gray-500 text-sm">
          &copy; {{ new Date().getFullYear() }} ForgeCommerce. All rights reserved.
        </p>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
const cartStore = useCartStore()
const authStore = useAuthStore()
</script>
