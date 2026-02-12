-- 016_stock_movements.up.sql
-- Audit trail for all inventory changes (both product variants and raw materials)

CREATE TABLE stock_movements (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    entity_type TEXT NOT NULL CHECK (entity_type IN ('product_variant', 'raw_material')),
    entity_id UUID NOT NULL,
    movement_type TEXT NOT NULL CHECK (movement_type IN ('purchase', 'sale', 'adjustment', 'production_consume', 'production_output', 'return', 'damage')),
    quantity_change NUMERIC(12,4) NOT NULL,
    quantity_before NUMERIC(12,4) NOT NULL,
    quantity_after NUMERIC(12,4) NOT NULL,
    reference_type TEXT,                             -- e.g., "order", "production_run", "manual"
    reference_id UUID,
    unit_cost NUMERIC(12,4),
    notes TEXT,
    created_by UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_stock_movements_entity ON stock_movements(entity_type, entity_id);
CREATE INDEX idx_stock_movements_movement_type ON stock_movements(movement_type);
CREATE INDEX idx_stock_movements_created_at ON stock_movements(created_at);
CREATE INDEX idx_stock_movements_created_by ON stock_movements(created_by);
