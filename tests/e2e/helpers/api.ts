const API_URL = process.env.API_URL || 'http://localhost:8080'

interface ProductData {
  id: string
  name: string
  slug: string
  base_price: string
  status: string
}

interface CartData {
  id: string
  items: unknown[]
  subtotal: string
  total: string
}

/**
 * Create a product via the admin API for test setup.
 * Requires a valid admin session token.
 */
export async function createTestProduct(
  name: string,
  price: string,
  adminToken?: string
): Promise<ProductData> {
  const slug = name.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')
  const res = await fetch(`${API_URL}/api/v1/admin/products`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(adminToken ? { Authorization: `Bearer ${adminToken}` } : {}),
    },
    body: JSON.stringify({
      name,
      slug,
      base_price: price,
      status: 'active',
    }),
  })
  if (!res.ok) {
    throw new Error(`Failed to create test product: ${res.status} ${await res.text()}`)
  }
  return res.json()
}

/**
 * Create an empty cart via the public API.
 */
export async function createTestCart(): Promise<CartData> {
  const res = await fetch(`${API_URL}/api/v1/cart`, { method: 'POST' })
  if (!res.ok) {
    throw new Error(`Failed to create test cart: ${res.status} ${await res.text()}`)
  }
  return res.json()
}

/**
 * Add an item to a cart.
 */
export async function addCartItem(
  cartId: string,
  variantId: string,
  quantity: number
): Promise<CartData> {
  const res = await fetch(`${API_URL}/api/v1/cart/${cartId}/items`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ variant_id: variantId, quantity }),
  })
  if (!res.ok) {
    throw new Error(`Failed to add cart item: ${res.status} ${await res.text()}`)
  }
  return res.json()
}

/**
 * Authenticate as admin and return the session cookie or token.
 */
export async function adminLogin(
  email = 'admin@forgecommerce.local',
  password = 'admin123'
): Promise<string> {
  const res = await fetch(`${API_URL}/admin/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!res.ok) {
    throw new Error(`Admin login failed: ${res.status} ${await res.text()}`)
  }
  const setCookie = res.headers.get('set-cookie')
  if (setCookie) {
    return setCookie.split(';')[0]
  }
  const data = await res.json()
  return data.token || ''
}

/**
 * Validate a VAT number via the public API.
 */
export async function validateVATNumber(
  cartId: string,
  vatNumber: string
): Promise<{ valid: boolean; company_name?: string }> {
  const res = await fetch(`${API_URL}/api/v1/cart/${cartId}/vat-number`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ vat_number: vatNumber }),
  })
  if (!res.ok) {
    throw new Error(`VAT validation failed: ${res.status} ${await res.text()}`)
  }
  return res.json()
}

/**
 * Get enabled selling countries from the public API.
 */
export async function getEnabledCountries(): Promise<
  Array<{ country_code: string; name: string }>
> {
  const res = await fetch(`${API_URL}/api/v1/countries`)
  if (!res.ok) {
    throw new Error(`Failed to get countries: ${res.status} ${await res.text()}`)
  }
  return res.json()
}
