-- 007_admin_users_sessions_audit.down.sql
-- Drop in reverse order of creation to respect FK dependencies

DROP TABLE IF EXISTS admin_audit_log;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS admin_users;
