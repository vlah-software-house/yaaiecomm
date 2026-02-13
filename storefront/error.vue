<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-50 px-4">
    <div class="text-center max-w-md">
      <p class="text-6xl font-bold text-primary-600">{{ error?.statusCode || 500 }}</p>
      <h1 class="mt-4 text-2xl font-bold text-gray-900">
        {{ is404 ? 'Page Not Found' : 'Something Went Wrong' }}
      </h1>
      <p class="mt-2 text-gray-600">
        {{ is404
          ? "The page you're looking for doesn't exist or has been moved."
          : error?.message || 'An unexpected error occurred. Please try again later.'
        }}
      </p>
      <div class="mt-8 flex flex-col sm:flex-row items-center justify-center gap-3">
        <NuxtLink
          to="/"
          class="inline-flex items-center px-5 py-2.5 bg-primary-600 text-white font-medium rounded-lg hover:bg-primary-700 transition-colors"
        >
          Go Home
        </NuxtLink>
        <button
          v-if="!is404"
          class="inline-flex items-center px-5 py-2.5 border border-gray-300 bg-white text-gray-700 font-medium rounded-lg hover:bg-gray-50 transition-colors"
          @click="handleError"
        >
          Try Again
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { NuxtError } from '#app'

const props = defineProps<{ error: NuxtError }>()
const is404 = computed(() => props.error?.statusCode === 404)

function handleError() {
  clearError({ redirect: '/' })
}

useHead({
  title: props.error?.statusCode === 404 ? 'Page Not Found' : 'Error',
})
</script>
