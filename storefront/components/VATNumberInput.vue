<script setup lang="ts">
import { ref, computed } from 'vue'
import type { VATValidation } from '~/types'

const props = defineProps<{
  cartId: string
}>()

const emit = defineEmits<{
  validated: [result: VATValidation]
}>()

const { post } = useApi()

const vatNumber = ref('')
const validating = ref(false)
const result = ref<VATValidation | null>(null)
const error = ref<string | null>(null)

// Format input: strip spaces, uppercase.
const formattedVatNumber = computed(() => {
  return vatNumber.value.replace(/\s+/g, '').toUpperCase()
})

const canValidate = computed(() => {
  return formattedVatNumber.value.length >= 4 && !validating.value
})

async function validate() {
  if (!canValidate.value) return

  validating.value = true
  error.value = null
  result.value = null

  try {
    const response = await post<VATValidation>(
      `/api/v1/cart/${props.cartId}/vat-number`,
      { vat_number: formattedVatNumber.value }
    )
    result.value = response
    emit('validated', response)
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Validation failed'
  } finally {
    validating.value = false
  }
}

function clear() {
  vatNumber.value = ''
  result.value = null
  error.value = null
}
</script>

<template>
  <div class="space-y-3">
    <label class="block text-sm font-medium text-gray-700">
      EU VAT Number
    </label>

    <p class="text-xs text-gray-500">
      Enter your VAT number for reverse charge on intra-EU B2B purchases.
    </p>

    <div class="flex gap-2">
      <input
        v-model="vatNumber"
        type="text"
        placeholder="e.g. DE123456789"
        :disabled="validating"
        class="block flex-1 rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 shadow-sm placeholder:text-gray-400 focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500 disabled:cursor-not-allowed disabled:bg-gray-100"
        @keydown.enter="validate"
      >
      <button
        type="button"
        :disabled="!canValidate"
        class="inline-flex items-center rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm transition-colors hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-1 disabled:cursor-not-allowed disabled:bg-gray-300 disabled:text-gray-500"
        @click="validate"
      >
        <svg
          v-if="validating"
          class="-ml-0.5 mr-1.5 h-4 w-4 animate-spin"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        {{ validating ? 'Validating...' : 'Validate' }}
      </button>
    </div>

    <!-- Validation result: valid -->
    <div
      v-if="result?.valid"
      class="flex items-start gap-2 rounded-md border border-green-200 bg-green-50 p-3"
    >
      <svg class="mt-0.5 h-4 w-4 flex-shrink-0 text-green-600" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.857-9.809a.75.75 0 00-1.214-.882l-3.483 4.79-1.88-1.88a.75.75 0 10-1.06 1.06l2.5 2.5a.75.75 0 001.137-.089l4-5.5z" clip-rule="evenodd" />
      </svg>
      <div class="min-w-0 flex-1">
        <p class="text-sm font-medium text-green-800">
          Valid VAT Number
        </p>
        <p v-if="result.company_name" class="mt-0.5 text-xs text-green-700">
          {{ result.company_name }}
        </p>
        <p v-if="result.address" class="text-xs text-green-600">
          {{ result.address }}
        </p>
        <button
          type="button"
          class="mt-1 text-xs text-green-700 underline hover:text-green-900"
          @click="clear"
        >
          Clear
        </button>
      </div>
    </div>

    <!-- Validation result: invalid -->
    <div
      v-if="result && !result.valid"
      class="flex items-start gap-2 rounded-md border border-red-200 bg-red-50 p-3"
    >
      <svg class="mt-0.5 h-4 w-4 flex-shrink-0 text-red-600" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.28 7.22a.75.75 0 00-1.06 1.06L8.94 10l-1.72 1.72a.75.75 0 101.06 1.06L10 11.06l1.72 1.72a.75.75 0 101.06-1.06L11.06 10l1.72-1.72a.75.75 0 00-1.06-1.06L10 8.94 8.28 7.22z" clip-rule="evenodd" />
      </svg>
      <div>
        <p class="text-sm font-medium text-red-800">
          Invalid VAT Number
        </p>
        <p v-if="result.message" class="mt-0.5 text-xs text-red-600">
          {{ result.message }}
        </p>
      </div>
    </div>

    <!-- API error -->
    <p v-if="error" class="text-xs text-red-600">
      {{ error }}
    </p>
  </div>
</template>
