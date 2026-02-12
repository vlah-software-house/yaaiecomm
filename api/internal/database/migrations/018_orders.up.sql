-- 018_orders.up.sql
-- Orders, order items (with VAT snapshots), and order event history

-- Sequential order number sequence starting at 1001
CREATE SEQUENCE IF NOT EXISTS order_number_seq START WITH 1001;

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number BIGINT NOT NULL DEFAULT nextval('order_number_seq'),
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'processing', 'shipped', 'delivered', 'cancelled', 'refunded')),
    email TEXT NOT NULL,
    billing_address JSONB NOT NULL,
    shipping_address JSONB NOT NULL,

    -- Monetary totals
    subtotal NUMERIC(12,2) NOT NULL DEFAULT 0,
    shipping_fee NUMERIC(12,2) NOT NULL DEFAULT 0,
    shipping_extra_fees NUMERIC(12,2) NOT NULL DEFAULT 0,
    discount_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    vat_total NUMERIC(12,2) NOT NULL DEFAULT 0,
    total NUMERIC(12,2) NOT NULL DEFAULT 0,

    -- VAT snapshot
    vat_number TEXT,                                 -- customer's VAT number if B2B
    vat_company_name TEXT,                           -- from VIES validation
    vat_reverse_charge BOOLEAN NOT NULL DEFAULT false,
    vat_country_code TEXT REFERENCES eu_countries(country_code),

    -- Stripe payment
    stripe_payment_intent_id TEXT,
    stripe_checkout_session_id TEXT,
    payment_status TEXT NOT NULL DEFAULT 'unpaid' CHECK (payment_status IN ('unpaid', 'paid', 'refunded', 'partially_refunded')),

    -- Discounts
    discount_id UUID,                                -- FK added when discounts table exists
    coupon_id UUID,                                  -- FK added when coupons table exists
    discount_breakdown JSONB,

    -- Shipping
    shipping_method TEXT,
    tracking_number TEXT,
    shipped_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,

    -- Notes and metadata
    notes TEXT,
    customer_notes TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX idx_orders_order_number ON orders(order_number);
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_payment_status ON orders(payment_status);
CREATE INDEX idx_orders_created_at ON orders(created_at);
CREATE INDEX idx_orders_email ON orders(email);
CREATE INDEX idx_orders_vat_country_code ON orders(vat_country_code);

-- Order line items with VAT snapshots
CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id UUID REFERENCES products(id) ON DELETE SET NULL,
    variant_id UUID REFERENCES product_variants(id) ON DELETE SET NULL,

    -- Snapshot data (preserved even if product/variant deleted)
    product_name TEXT NOT NULL,
    variant_name TEXT,
    variant_options JSONB,                           -- snapshot of selected options
    sku TEXT,

    -- Pricing
    quantity INTEGER NOT NULL DEFAULT 1,
    unit_price NUMERIC(12,2) NOT NULL,
    total_price NUMERIC(12,2) NOT NULL,

    -- VAT snapshot
    vat_rate NUMERIC(5,2) NOT NULL DEFAULT 0,
    vat_rate_type TEXT,
    vat_amount NUMERIC(12,2) NOT NULL DEFAULT 0,
    price_includes_vat BOOLEAN NOT NULL DEFAULT true,
    net_unit_price NUMERIC(12,2) NOT NULL,
    gross_unit_price NUMERIC(12,2) NOT NULL,

    -- Shipping
    weight_grams INTEGER,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
CREATE INDEX idx_order_items_product_id ON order_items(product_id);
CREATE INDEX idx_order_items_variant_id ON order_items(variant_id);

-- Order event history (status transitions, notes, actions)
CREATE TABLE order_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL,                        -- e.g., "status_changed", "payment_received", "note_added"
    from_status TEXT,
    to_status TEXT,
    data JSONB,
    created_by UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_events_order_id ON order_events(order_id);
CREATE INDEX idx_order_events_event_type ON order_events(event_type);
CREATE INDEX idx_order_events_created_at ON order_events(created_at);
