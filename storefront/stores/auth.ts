import { defineStore } from 'pinia'
import type { AuthResponse, CustomerProfile } from '~/types'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    customer: null as CustomerProfile | null,
    accessToken: null as string | null,
    refreshToken: null as string | null,
  }),

  getters: {
    isAuthenticated: (state) => !!state.accessToken,
    displayName: (state) => {
      if (!state.customer) return ''
      const parts = [state.customer.first_name, state.customer.last_name].filter(Boolean)
      return parts.length ? parts.join(' ') : state.customer.email
    },
  },

  actions: {
    async register(email: string, password: string, firstName?: string, lastName?: string) {
      const { post } = useApi()
      const res = await post<AuthResponse>('/api/v1/customers/register', {
        email,
        password,
        first_name: firstName || null,
        last_name: lastName || null,
      })
      this.setAuth(res)
    },

    async login(email: string, password: string) {
      const { post } = useApi()
      const res = await post<AuthResponse>('/api/v1/customers/login', {
        email,
        password,
      })
      this.setAuth(res)
    },

    async refresh() {
      if (!this.refreshToken) return
      const { post } = useApi()
      try {
        const res = await post<{ access_token: string; refresh_token: string }>(
          '/api/v1/customers/refresh',
          { refresh_token: this.refreshToken }
        )
        this.accessToken = res.access_token
        this.refreshToken = res.refresh_token
        if (import.meta.client) {
          localStorage.setItem('access_token', res.access_token)
          localStorage.setItem('refresh_token', res.refresh_token)
        }
      } catch {
        this.logout()
      }
    },

    logout() {
      this.customer = null
      this.accessToken = null
      this.refreshToken = null
      if (import.meta.client) {
        localStorage.removeItem('access_token')
        localStorage.removeItem('refresh_token')
      }
    },

    setAuth(res: AuthResponse) {
      this.accessToken = res.access_token
      this.refreshToken = res.refresh_token
      this.customer = res.customer
      if (import.meta.client) {
        localStorage.setItem('access_token', res.access_token)
        localStorage.setItem('refresh_token', res.refresh_token)
      }
    },

    /** Restore auth state from localStorage on page load. */
    restoreAuth() {
      if (!import.meta.client) return
      this.accessToken = localStorage.getItem('access_token')
      this.refreshToken = localStorage.getItem('refresh_token')
    },
  },
})
