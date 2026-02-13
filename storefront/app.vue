<template>
  <NuxtLayout>
    <NuxtPage />
  </NuxtLayout>
</template>

<script setup lang="ts">
const cartStore = useCartStore()
const authStore = useAuthStore()

onMounted(() => {
  authStore.restoreAuth()
  cartStore.restoreCart()
})
</script>
