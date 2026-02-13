<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { ProductAttribute, ProductVariant } from '~/types'

const props = defineProps<{
  attributes: ProductAttribute[]
  variants: ProductVariant[]
}>()

const emit = defineEmits<{
  select: [variant: ProductVariant]
}>()

// Track selected option value per attribute name.
const selections = ref<Record<string, string>>({})

// Initialize with the first option of each attribute.
function initSelections() {
  const sel: Record<string, string> = {}
  for (const attr of props.attributes) {
    if (attr.options.length > 0) {
      sel[attr.name] = attr.options[0].value
    }
  }
  selections.value = sel
}
initSelections()

// Find the variant matching the current selections.
const selectedVariant = computed<ProductVariant | null>(() => {
  return props.variants.find((v) => {
    return v.options.every(
      (opt) => selections.value[opt.attribute_name] === opt.option_value
    )
  }) ?? null
})

// Check whether a specific option value for an attribute leads to any
// in-stock variant given the other current selections.
function isOptionAvailable(attributeName: string, optionValue: string): boolean {
  const hypothetical = { ...selections.value, [attributeName]: optionValue }
  return props.variants.some((v) => {
    const matches = v.options.every(
      (opt) => hypothetical[opt.attribute_name] === opt.option_value
    )
    return matches && v.is_active && v.stock_quantity > 0
  })
}

function selectOption(attributeName: string, optionValue: string) {
  selections.value = { ...selections.value, [attributeName]: optionValue }
}

function formatPrice(price: string | null, fallback?: string): string {
  const raw = price ?? fallback
  if (!raw) return ''
  const num = parseFloat(raw)
  return `€${num.toFixed(2)}`
}

// Emit the selected variant whenever it changes.
watch(selectedVariant, (variant) => {
  if (variant) {
    emit('select', variant)
  }
}, { immediate: true })
</script>

<template>
  <div class="space-y-6">
    <!-- Attribute groups -->
    <div
      v-for="attr in attributes"
      :key="attr.id"
    >
      <label class="mb-2 block text-sm font-medium text-gray-700">
        {{ attr.display_name }}
      </label>

      <div class="flex flex-wrap gap-2">
        <!-- Color swatches -->
        <template v-if="attr.attribute_type === 'color_swatch'">
          <button
            v-for="opt in attr.options"
            :key="opt.id"
            type="button"
            :title="opt.display_value"
            :aria-label="opt.display_value"
            class="relative h-9 w-9 rounded-full border-2 transition-all focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-1"
            :class="[
              selections[attr.name] === opt.value
                ? 'border-indigo-600 ring-1 ring-indigo-600'
                : 'border-gray-300 hover:border-gray-400',
              !isOptionAvailable(attr.name, opt.value) ? 'opacity-40 cursor-not-allowed' : 'cursor-pointer'
            ]"
            @click="selectOption(attr.name, opt.value)"
          >
            <span
              class="absolute inset-1 rounded-full"
              :style="{ backgroundColor: opt.color_hex || '#ccc' }"
            />
            <!-- Out-of-stock strike-through line -->
            <span
              v-if="!isOptionAvailable(attr.name, opt.value)"
              class="absolute inset-0 flex items-center justify-center"
            >
              <span class="block h-px w-full rotate-45 bg-gray-500" />
            </span>
          </button>
        </template>

        <!-- Button options (default for select, button_group, image_swatch) -->
        <template v-else>
          <button
            v-for="opt in attr.options"
            :key="opt.id"
            type="button"
            class="rounded-md border px-4 py-2 text-sm font-medium transition-colors focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:ring-offset-1"
            :class="[
              selections[attr.name] === opt.value
                ? 'border-indigo-600 bg-indigo-50 text-indigo-700'
                : 'border-gray-300 bg-white text-gray-700 hover:bg-gray-50',
              !isOptionAvailable(attr.name, opt.value) ? 'opacity-40 line-through cursor-not-allowed' : 'cursor-pointer'
            ]"
            @click="selectOption(attr.name, opt.value)"
          >
            {{ opt.display_value }}
          </button>
        </template>
      </div>
    </div>

    <!-- Selected variant info -->
    <div
      v-if="selectedVariant"
      class="rounded-md border border-gray-200 bg-gray-50 p-4"
    >
      <div class="flex items-center justify-between">
        <div>
          <span class="text-lg font-semibold text-gray-900">
            {{ formatPrice(selectedVariant.price, '') }}
          </span>
          <span
            v-if="selectedVariant.compare_at_price"
            class="ml-2 text-sm text-gray-400 line-through"
          >
            {{ formatPrice(selectedVariant.compare_at_price) }}
          </span>
        </div>

        <div>
          <span
            v-if="!selectedVariant.is_active || selectedVariant.stock_quantity <= 0"
            class="inline-flex items-center rounded-full bg-red-100 px-2.5 py-0.5 text-xs font-medium text-red-700"
          >
            Out of Stock
          </span>
          <span
            v-else-if="selectedVariant.stock_quantity <= 5"
            class="inline-flex items-center rounded-full bg-amber-100 px-2.5 py-0.5 text-xs font-medium text-amber-700"
          >
            Only {{ selectedVariant.stock_quantity }} left
          </span>
          <span
            v-else
            class="inline-flex items-center rounded-full bg-green-100 px-2.5 py-0.5 text-xs font-medium text-green-700"
          >
            In Stock
          </span>
        </div>
      </div>

      <p class="mt-1 text-xs text-gray-500">
        SKU: {{ selectedVariant.sku }}
      </p>
    </div>

    <div
      v-else
      class="rounded-md border border-amber-200 bg-amber-50 p-4 text-sm text-amber-700"
    >
      This combination is not available.
    </div>
  </div>
</template>
