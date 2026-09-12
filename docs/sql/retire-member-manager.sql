-- User approved: make Petugas Anggota accounts roleless and cancel open requests.
-- Direct SQL for the existing schema; no migration or account deletion.
BEGIN;
SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(7411, 1);
LOCK TABLE admin_users, tickets IN SHARE ROW EXCLUSIVE MODE;

DO $$
BEGIN
  IF (SELECT count(*) FROM admin_users WHERE role_code = 'member_manager') <> 1 THEN
    RAISE EXCEPTION 'Petugas Anggota account count changed; inspect before retrying';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM admin_users WHERE role_code = 'super_admin' AND is_active) THEN
    RAISE EXCEPTION 'An active Super Admin is required';
  END IF;
END $$;

CREATE TEMP TABLE retired_role_accounts ON COMMIT DROP AS
SELECT id, role_code = 'member_manager' AS affected,
  CASE WHEN role_code = 'member_manager'
    THEN md5((to_jsonb(a) - 'role_code' - 'updated_at')::text)
    ELSE md5(to_jsonb(a)::text)
  END AS preserved_fields
FROM admin_users a;
CREATE TEMP TABLE retired_role_tickets ON COMMIT DROP AS
SELECT id, status = 'open' AND requested_role_code = 'member_manager' AS affected,
  CASE WHEN status = 'open' AND requested_role_code = 'member_manager'
    THEN md5((to_jsonb(t) - 'status' - 'cancelled_at' - 'updated_at')::text)
    ELSE md5(to_jsonb(t)::text)
  END AS preserved_fields
FROM tickets t;

UPDATE admin_users SET role_code = NULL, updated_at = now()
WHERE role_code = 'member_manager';
UPDATE tickets SET status = 'cancelled', cancelled_at = now(), updated_at = now()
WHERE status = 'open' AND requested_role_code = 'member_manager';

DO $$
BEGIN
  IF (SELECT count(*) FROM admin_users) <> (SELECT count(*) FROM retired_role_accounts)
     OR EXISTS (SELECT 1 FROM admin_users a JOIN retired_role_accounts b USING(id)
       WHERE CASE WHEN b.affected
         THEN md5((to_jsonb(a) - 'role_code' - 'updated_at')::text)
         ELSE md5(to_jsonb(a)::text) END <> b.preserved_fields
       OR (b.affected AND a.role_code IS NOT NULL)) THEN
    RAISE EXCEPTION 'Account identity, credentials, active status, or unrelated fields changed';
  END IF;
  IF (SELECT count(*) FROM tickets) <> (SELECT count(*) FROM retired_role_tickets)
     OR EXISTS (SELECT 1 FROM tickets t JOIN retired_role_tickets b USING(id)
       WHERE CASE WHEN b.affected
         THEN md5((to_jsonb(t) - 'status' - 'cancelled_at' - 'updated_at')::text)
         ELSE md5(to_jsonb(t)::text) END <> b.preserved_fields
       OR (b.affected AND (t.status <> 'cancelled' OR t.cancelled_at IS NULL))) THEN
    RAISE EXCEPTION 'Ticket reason, history, or unrelated fields changed';
  END IF;
END $$;

SELECT count(*)::integer AS accounts_made_roleless FROM retired_role_accounts WHERE affected;
SELECT count(*)::integer AS requests_cancelled FROM retired_role_tickets WHERE affected;
COMMIT;
