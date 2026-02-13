/**
 * Composable for making API calls to the ForgeCommerce Go backend.
 * Handles base URL configuration and common options.
 */
export function useApi() {
  const config = useRuntimeConfig()
  const baseURL = config.public.apiBase as string

  async function apiFetch<T>(path: string, options: RequestInit = {}): Promise<T> {
    const url = `${baseURL}${path}`

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(options.headers as Record<string, string> || {}),
    }

    // Add auth token if available.
    if (import.meta.client) {
      const token = localStorage.getItem('access_token')
      if (token) {
        headers['Authorization'] = `Bearer ${token}`
      }
    }

    const response = await fetch(url, {
      ...options,
      headers,
    })

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({ error: 'Unknown error' }))
      throw new ApiError(response.status, errorBody.error || 'Request failed')
    }

    return response.json()
  }

  function get<T>(path: string): Promise<T> {
    return apiFetch<T>(path, { method: 'GET' })
  }

  function post<T>(path: string, body?: unknown): Promise<T> {
    return apiFetch<T>(path, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  function patch<T>(path: string, body?: unknown): Promise<T> {
    return apiFetch<T>(path, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  function del<T>(path: string): Promise<T> {
    return apiFetch<T>(path, { method: 'DELETE' })
  }

  return { get, post, patch, del, apiFetch }
}

export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}
