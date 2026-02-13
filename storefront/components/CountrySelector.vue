<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { Country } from '~/types'

defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const { get } = useApi()

const countries = ref<Country[]>([])
const loading = ref(true)
const error = ref<string | null>(null)

onMounted(async () => {
  try {
    countries.value = await get<Country[]>('/api/v1/countries')
  } catch (e) {
    error.value = 'Failed to load countries'
    console.error('CountrySelector: failed to fetch countries', e)
  } finally {
    loading.value = false
  }
})

function onChange(event: Event) {
  const target = event.target as HTMLSelectElement
  emit('update:modelValue', target.value)
}
</script>

<template>
  <div>
    <select
      :value="modelValue"
      :disabled="loading"
      class="block w-full rounded-md border border-gray-300 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm transition-colors focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500"
      @change="onChange"
    >
      <option value="" disabled>
        {{ loading ? 'Loading countries...' : 'Select a country' }}
      </option>
      <option
        v-for="country in countries"
        :key="country.country_code"
        :value="country.country_code"
      >
        {{ country.name }} ({{ country.country_code }})
      </option>
    </select>

    <p
      v-if="error"
      class="mt-1 text-xs text-red-600"
    >
      {{ error }}
    </p>
  </div>
</template>
