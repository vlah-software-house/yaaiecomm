<script setup lang="ts">
import type { CheckoutCalculation } from '~/types'

const props = defineProps<{
  calculation: CheckoutCalculation | null
}>()

function formatPrice(price: string): string {
  const num = parseFloat(price)
  return `€${num.toFixed(2)}`
}
</script>

<template>
  <div v-if="calculation" class="rounded-lg border border-gray-200 bg-white p-5">
    <h3 class="mb-4 text-sm font-semibold uppercase tracking-wide text-gray-500">
      Order Summary
    </h3>

    <dl class="space-y-3">
      <!-- Subtotal -->
      <div class="flex items-center justify-between">
        <dt class="text-sm text-gray-600">Subtotal</dt>
        <dd class="text-sm font-medium text-gray-900">
          {{ formatPrice(calculation.subtotal) }}
        </dd>
      </div>

      <!-- VAT -->
      <div class="flex items-center justify-between">
        <dt class="text-sm text-gray-600">
          <template v-if="calculation.reverse_charge">
            VAT (Reverse Charge &mdash; 0%)
          </template>
          <template v-else>
            VAT
          </template>
        </dt>
        <dd class="text-sm font-medium text-gray-900">
          {{ formatPrice(calculation.vat_total) }}
        </dd>
      </div>

      <!-- VAT breakdown per item (when not reverse charge) -->
      <template v-if="!calculation.reverse_charge && calculation.vat_breakdown.length > 0">
        <div
          v-for="(item, idx) in calculation.vat_breakdown"
          :key="idx"
          class="flex items-center justify-between pl-4"
        >
          <dt class="text-xs text-gray-400">
            {{ item.product_name }} ({{ item.rate }}% {{ item.rate_type }})
          </dt>
          <dd class="text-xs text-gray-500">
            {{ formatPrice(item.amount) }}
          </dd>
        </div>
      </template>

      <!-- Reverse charge notice -->
      <div
        v-if="calculation.reverse_charge"
        class="rounded-md bg-blue-50 px-3 py-2"
      >
        <p class="text-xs text-blue-700">
          VAT reverse charge applies. As the buyer, you are liable for VAT in your country of registration.
        </p>
      </div>

      <!-- Shipping -->
      <div class="flex items-center justify-between">
        <dt class="text-sm text-gray-600">Shipping</dt>
        <dd class="text-sm font-medium text-gray-900">
          <template v-if="parseFloat(calculation.shipping_fee) === 0">
            Free
          </template>
          <template v-else>
            {{ formatPrice(calculation.shipping_fee) }}
          </template>
        </dd>
      </div>

      <!-- Discount -->
      <div
        v-if="parseFloat(calculation.discount_amount) > 0"
        class="flex items-center justify-between"
      >
        <dt class="text-sm text-green-600">Discount</dt>
        <dd class="text-sm font-medium text-green-600">
          -{{ formatPrice(calculation.discount_amount) }}
        </dd>
      </div>

      <!-- Divider -->
      <div class="border-t border-gray-200" />

      <!-- Total -->
      <div class="flex items-center justify-between">
        <dt class="text-base font-semibold text-gray-900">Total</dt>
        <dd class="text-base font-semibold text-gray-900">
          {{ formatPrice(calculation.total) }}
        </dd>
      </div>
    </dl>
  </div>
</template>
