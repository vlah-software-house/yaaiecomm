-- Production batch management
CREATE TYPE production_batch_status AS ENUM ('draft', 'scheduled', 'in_progress', 'completed', 'cancelled');

CREATE TABLE production_batches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_number TEXT NOT NULL UNIQUE,
    product_id UUID NOT NULL REFERENCES products(id),
    variant_id UUID REFERENCES product_variants(id),
    planned_quantity INT NOT NULL CHECK (planned_quantity > 0),
    actual_quantity INT DEFAULT 0,
    status production_batch_status NOT NULL DEFAULT 'draft',
    scheduled_date DATE,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    notes TEXT,
    cost_total NUMERIC(12,2) DEFAULT 0,
    created_by UUID REFERENCES admin_users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_production_batches_status ON production_batches(status);
CREATE INDEX idx_production_batches_product ON production_batches(product_id);
CREATE INDEX idx_production_batches_scheduled ON production_batches(scheduled_date);

-- Production batch materials (resolved BOM snapshot)
CREATE TABLE production_batch_materials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id UUID NOT NULL REFERENCES production_batches(id) ON DELETE CASCADE,
    raw_material_id UUID NOT NULL REFERENCES raw_materials(id),
    required_quantity NUMERIC(12,4) NOT NULL,
    consumed_quantity NUMERIC(12,4) DEFAULT 0,
    unit_cost NUMERIC(12,4) DEFAULT 0,
    UNIQUE(batch_id, raw_material_id)
);
