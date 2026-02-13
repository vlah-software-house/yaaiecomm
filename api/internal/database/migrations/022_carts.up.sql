-- 022_carts.up.sql
-- Shopping carts with anonymous/authenticated support, VAT number for B2B, and country-aware pricing

CREATE TABLE carts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    email TEXT,
    country_code TEXT REFERENCES eu_countries(country_code),
    vat_number TEXT,
    coupon_code TEXT,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '7 days',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE cart_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    cart_id UUID NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    variant_id UUID NOT NULL REFERENCES product_variants(id),
    quantity INTEGER NOT NULL DEFAULT 1 CHECK (quantity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(cart_id, variant_id)
);

CREATE INDEX idx_carts_customer_id ON carts(customer_id) WHERE customer_id IS NOT NULL;
CREATE INDEX idx_carts_expires_at ON carts(expires_at);
CREATE INDEX idx_cart_items_cart_id ON cart_items(cart_id);
