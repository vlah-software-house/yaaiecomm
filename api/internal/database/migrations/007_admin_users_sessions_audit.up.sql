-- 007_admin_users_sessions_audit.up.sql
-- Admin users, sessions, and audit logging

-- Admin users table
CREATE TABLE admin_users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,                -- bcrypt, cost 12+
    role TEXT NOT NULL DEFAULT 'admin',
    permissions TEXT[] NOT NULL DEFAULT '{}',

    -- TOTP 2FA
    totp_secret TEXT,                           -- encrypted at rest
    totp_verified BOOLEAN NOT NULL DEFAULT false,
    recovery_codes TEXT[],                      -- hashed recovery codes
    force_2fa_setup BOOLEAN NOT NULL DEFAULT true,

    is_active BOOLEAN NOT NULL DEFAULT true,
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for active user lookups and login
CREATE INDEX idx_admin_users_email_active ON admin_users(email) WHERE is_active = true;
CREATE INDEX idx_admin_users_role ON admin_users(role);

-- Sessions table (server-side session storage)
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,                        -- session token (cryptographically random)
    admin_user_id UUID NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
    data JSONB NOT NULL DEFAULT '{}',
    ip_address TEXT,
    user_agent TEXT,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for session lookups and cleanup
CREATE INDEX idx_sessions_admin_user_id ON sessions(admin_user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- Admin audit log
CREATE TABLE admin_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_user_id UUID REFERENCES admin_users(id) ON DELETE SET NULL,
    action TEXT NOT NULL,                       -- e.g., "product.created", "order.status_changed", "vat_rates.auto_updated"
    entity_type TEXT,                           -- e.g., "product", "order", "vat_rate", "store_settings"
    entity_id TEXT,                             -- string to accommodate both UUIDs and other ID formats
    changes JSONB,                              -- before/after or details of the change
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for audit log queries
CREATE INDEX idx_audit_log_admin_user_id ON admin_audit_log(admin_user_id);
CREATE INDEX idx_audit_log_created_at ON admin_audit_log(created_at);
CREATE INDEX idx_audit_log_entity ON admin_audit_log(entity_type, entity_id);
CREATE INDEX idx_audit_log_action ON admin_audit_log(action);
