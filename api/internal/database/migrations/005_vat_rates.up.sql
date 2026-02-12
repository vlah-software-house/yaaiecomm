-- 005_vat_rates.up.sql
-- VAT rates per EU country, synced from EC TEDB / euvatrates.com

CREATE TABLE vat_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country_code TEXT NOT NULL REFERENCES eu_countries(country_code),
    rate_type TEXT NOT NULL,
    rate NUMERIC(5,2) NOT NULL,
    description TEXT,
    valid_from DATE NOT NULL,
    valid_to DATE,                              -- NULL = currently active
    source TEXT NOT NULL DEFAULT 'seed',         -- "ec_tedb", "euvatrates_json", "manual", "seed"
    synced_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT chk_vat_rates_rate_type CHECK (
        rate_type IN ('standard', 'reduced', 'reduced_alt', 'super_reduced', 'parking', 'zero')
    ),
    CONSTRAINT uq_vat_rates_country_type_from UNIQUE (country_code, rate_type, valid_from)
);

-- Primary lookup: current rate for a country + rate_type (valid_to IS NULL)
CREATE INDEX idx_vat_rates_current ON vat_rates(country_code, rate_type, valid_to)
    WHERE valid_to IS NULL;

-- General lookups by country
CREATE INDEX idx_vat_rates_country ON vat_rates(country_code);

-- Lookup by source for admin/audit
CREATE INDEX idx_vat_rates_source ON vat_rates(source);

-- Lookup by sync time
CREATE INDEX idx_vat_rates_synced_at ON vat_rates(synced_at);
