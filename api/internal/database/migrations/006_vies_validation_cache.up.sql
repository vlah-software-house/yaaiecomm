-- 006_vies_validation_cache.up.sql
-- Cache for VIES VAT number validation results

CREATE TABLE vies_validation_cache (
    vat_number TEXT PRIMARY KEY,               -- full VAT number including country prefix, e.g., "ES12345678A"
    is_valid BOOLEAN NOT NULL,
    company_name TEXT,
    company_address TEXT,
    consultation_number TEXT,                   -- VIES request identifier for audit trail
    validated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL             -- validated_at + 24h typically
);

-- Index for cleanup of expired entries
CREATE INDEX idx_vies_cache_expires_at ON vies_validation_cache(expires_at);
