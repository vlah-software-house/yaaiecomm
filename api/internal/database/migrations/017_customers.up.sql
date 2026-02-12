-- 017_customers.up.sql
-- Customer accounts with B2B VAT support and address book

CREATE TABLE customers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    first_name TEXT,
    last_name TEXT,
    phone TEXT,
    password_hash TEXT,                              -- bcrypt; NULL for guest checkout customers
    default_billing_address JSONB,
    default_shipping_address JSONB,
    accepts_marketing BOOLEAN NOT NULL DEFAULT false,
    stripe_customer_id TEXT UNIQUE,
    vat_number TEXT,                                  -- for B2B customers
    notes TEXT,
    metadata JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_customers_email ON customers(email);
CREATE INDEX idx_customers_stripe_customer_id ON customers(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;

-- Customer address book
CREATE TABLE customer_addresses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id UUID NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    label TEXT,                                      -- e.g., "Home", "Office"
    first_name TEXT NOT NULL,
    last_name TEXT NOT NULL,
    company TEXT,
    address_line1 TEXT NOT NULL,
    address_line2 TEXT,
    city TEXT NOT NULL,
    state_province TEXT,
    postal_code TEXT NOT NULL,
    country_code TEXT NOT NULL REFERENCES eu_countries(country_code),
    phone TEXT,
    is_default_billing BOOLEAN NOT NULL DEFAULT false,
    is_default_shipping BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_customer_addresses_customer_id ON customer_addresses(customer_id);
CREATE INDEX idx_customer_addresses_country_code ON customer_addresses(country_code);
