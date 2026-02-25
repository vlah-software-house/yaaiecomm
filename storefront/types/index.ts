// Product types matching the Go API responses.

export interface ProductImage {
  id: string
  url: string
  alt_text: string | null
  position: number
  is_primary: boolean
  variant_id?: string
}

export interface ProductSummary {
  id: string
  name: string
  slug: string
  sku_prefix: string | null
  base_price: string
  compare_at_price: string | null
  short_description: string | null
  status: string
  has_variants: boolean
  featured_image: ProductImage | null
  created_at: string
}

export interface ProductDetail {
  id: string
  name: string
  slug: string
  description: string | null
  short_description: string | null
  status: string
  sku_prefix: string | null
  base_price: string
  compare_at_price: string | null
  base_weight_grams: number
  has_variants: boolean
  seo_title?: string
  seo_description?: string
  metadata?: Record<string, unknown>
  created_at: string
  updated_at: string
  images: ProductImage[]
  attributes: ProductAttribute[]
  variants: ProductVariant[]
}

export interface ProductAttribute {
  id: string
  name: string
  display_name: string
  attribute_type: string
  position: number
  options: AttributeOption[]
}

export interface AttributeOption {
  id: string
  value: string
  display_value: string
  color_hex?: string
  price_modifier: string
  weight_modifier_grams?: number
  position: number
}

export interface ProductVariant {
  id: string
  sku: string
  price: string | null
  compare_at_price: string | null
  stock_quantity: number
  weight_grams?: number
  barcode?: string
  is_active: boolean
  position: number
  options: VariantOption[]
  images: ProductImage[]
}

export interface VariantOption {
  attribute_name: string
  option_value: string
  option_display_value: string
}

// Category
export interface Category {
  id: string
  name: string
  slug: string
  description?: string
  parent_id: string | null
  position: number
  image_url?: string
}

// Country (enabled shipping countries)
export interface Country {
  country_code: string
  name: string
}

// Cart types
export interface Cart {
  id: string
  email: string | null
  country_code: string | null
  vat_number: string | null
  coupon_code: string | null
  expires_at: string
  created_at: string
  items: CartItem[]
}

export interface CartItem {
  id: string
  cart_id: string
  variant_id: string
  quantity: number
  variant_sku: string
  variant_price: string
  variant_stock: number
  variant_is_active: boolean
  product_id: string
  product_name: string
  product_slug: string
  product_base_price: string
}

// Checkout calculation response
export interface CheckoutCalculation {
  subtotal: string
  vat_total: string
  shipping_fee: string
  discount_amount: string
  total: string
  vat_breakdown: VATBreakdownItem[]
  reverse_charge: boolean
}

export interface VATBreakdownItem {
  product_name: string
  rate: string
  rate_type: string
  amount: string
}

// VAT number validation response
export interface VATValidation {
  valid: boolean
  company_name?: string
  address?: string
  vat_number: string
  message?: string
}

// Auth types
export interface AuthResponse {
  access_token: string
  refresh_token: string
  customer: CustomerProfile
}

export interface CustomerProfile {
  id: string
  email: string
  first_name: string | null
  last_name: string | null
  phone: string | null
  vat_number?: string
}

// Paginated list response
export interface PaginatedResponse<T> {
  data: T[]
  page: number
  total_pages: number
  total: number
}
