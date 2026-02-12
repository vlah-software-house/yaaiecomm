-- 019_discounts.up.sql
-- Discount engine: discounts, coupons, and coupon usage tracking

CREATE TABLE discounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('percentage', 'fixed_amount')),
    value NUMERIC(12,2) NOT NULL,
    scope TEXT NOT NULL DEFAULT 'subtotal' CHECK (scope IN ('subtotal', 'shipping', 'total')),
    minimum_amount NUMERIC(12,2),
    maximum_discount NUMERIC(12,2),
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    priority INTEGER NOT NULL DEFAULT 0,
    stackable BOOLEAN NOT NULL DEFAULT false,
    conditions JSONB NOT NULL DEFAULT '{}',           -- flexible conditions: product IDs, category IDs, customer groups, etc.
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_discounts_is_active ON discounts(is_active);
CREATE INDEX idx_discounts_starts_at ON discounts(starts_at);
CREATE INDEX idx_discounts_ends_at ON discounts(ends_at);
CREATE INDEX idx_discounts_priority ON discounts(priority);

CREATE TABLE coupons (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code TEXT UNIQUE NOT NULL,
    discount_id UUID NOT NULL REFERENCES discounts(id) ON DELETE CASCADE,
    usage_limit INTEGER,                             -- total max uses, NULL = unlimited
    usage_limit_per_customer INTEGER,                -- max uses per customer, NULL = unlimited
    usage_count INTEGER NOT NULL DEFAULT 0,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_coupons_code ON coupons(code);
CREATE INDEX idx_coupons_discount_id ON coupons(discount_id);
CREATE INDEX idx_coupons_is_active ON coupons(is_active);

CREATE TABLE coupon_usage (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    coupon_id UUID NOT NULL REFERENCES coupons(id) ON DELETE CASCADE,
    customer_id UUID REFERENCES customers(id) ON DELETE SET NULL,
    order_id UUID REFERENCES orders(id) ON DELETE SET NULL,
    used_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_coupon_usage_coupon_id ON coupon_usage(coupon_id);
CREATE INDEX idx_coupon_usage_customer_id ON coupon_usage(customer_id);

-- Now add the FK constraints on orders for discount_id and coupon_id
ALTER TABLE orders
    ADD CONSTRAINT fk_orders_discount_id
    FOREIGN KEY (discount_id) REFERENCES discounts(id) ON DELETE SET NULL;

ALTER TABLE orders
    ADD CONSTRAINT fk_orders_coupon_id
    FOREIGN KEY (coupon_id) REFERENCES coupons(id) ON DELETE SET NULL;
