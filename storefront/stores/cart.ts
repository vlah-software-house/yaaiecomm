import { defineStore } from 'pinia'
import type { Cart, CartItem, CheckoutCalculation } from '~/types'

export const useCartStore = defineStore('cart', {
  state: () => ({
    cart: null as Cart | null,
    loading: false,
    error: null as string | null,
    calculation: null as CheckoutCalculation | null,
  }),

  getters: {
    cartId: (state) => state.cart?.id ?? null,
    items: (state) => state.cart?.items ?? [],
    itemCount: (state) =>
      state.cart?.items?.reduce((sum, item) => sum + item.quantity, 0) ?? 0,
    isEmpty: (state) => !state.cart?.items?.length,
  },

  actions: {
    async createCart() {
      const { post } = useApi()
      this.loading = true
      this.error = null
      try {
        this.cart = await post<Cart>('/api/v1/cart')
      } catch (e: any) {
        this.error = e.message
      } finally {
        this.loading = false
      }
    },

    async loadCart(id: string) {
      const { get } = useApi()
      this.loading = true
      this.error = null
      try {
        this.cart = await get<Cart>(`/api/v1/cart/${id}`)
      } catch (e: any) {
        this.error = e.message
        this.cart = null
      } finally {
        this.loading = false
      }
    },

    async addItem(variantId: string, quantity: number = 1) {
      if (!this.cart) {
        await this.createCart()
      }
      if (!this.cart) return

      const { post } = useApi()
      this.loading = true
      this.error = null
      try {
        await post(`/api/v1/cart/${this.cart.id}/items`, {
          variant_id: variantId,
          quantity,
        })
        await this.loadCart(this.cart.id)
      } catch (e: any) {
        this.error = e.message
      } finally {
        this.loading = false
      }
    },

    async updateItemQuantity(itemId: string, quantity: number) {
      if (!this.cart) return

      const { patch } = useApi()
      this.loading = true
      this.error = null
      try {
        await patch(`/api/v1/cart/${this.cart.id}/items/${itemId}`, { quantity })
        await this.loadCart(this.cart.id)
      } catch (e: any) {
        this.error = e.message
      } finally {
        this.loading = false
      }
    },

    async removeItem(itemId: string) {
      if (!this.cart) return

      const { del } = useApi()
      this.loading = true
      this.error = null
      try {
        await del(`/api/v1/cart/${this.cart.id}/items/${itemId}`)
        await this.loadCart(this.cart.id)
      } catch (e: any) {
        this.error = e.message
      } finally {
        this.loading = false
      }
    },

    async calculateTotals(countryCode: string, vatNumber?: string) {
      if (!this.cart) return

      const { post } = useApi()
      try {
        this.calculation = await post<CheckoutCalculation>(
          '/api/v1/checkout/calculate',
          {
            cart_id: this.cart.id,
            country_code: countryCode,
            vat_number: vatNumber || '',
          }
        )
      } catch (e: any) {
        this.error = e.message
        this.calculation = null
      }
    },

    /** Persist cart ID to localStorage for guest users. */
    persistCartId() {
      if (this.cart?.id && import.meta.client) {
        localStorage.setItem('cart_id', this.cart.id)
      }
    },

    /** Restore cart from localStorage on page load. */
    async restoreCart() {
      if (!import.meta.client) return
      const cartId = localStorage.getItem('cart_id')
      if (cartId && !this.cart) {
        await this.loadCart(cartId)
        if (!this.cart) {
          localStorage.removeItem('cart_id')
        }
      }
    },
  },
})
