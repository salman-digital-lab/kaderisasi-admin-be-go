-- Direct, guarded role consolidation approved for the September 2026 rollout.
-- Run against the intended existing schema; this is not an Ace migration.
BEGIN;
SET LOCAL lock_timeout = '10s';
SET LOCAL statement_timeout = '30s';
SELECT pg_advisory_xact_lock(7411, 1);
LOCK TABLE admin_users, tickets IN SHARE ROW EXCLUSIVE MODE;

DO $$
BEGIN
  IF (SELECT count(*) FROM admin_users WHERE role_code = 'asmen') <> 2
     OR (SELECT count(*) FROM admin_users WHERE role_code = 'kapro') <> 2
     OR (SELECT count(*) FROM admin_users WHERE role_code = 'leaderboard') <> 1 THEN
    RAISE EXCEPTION 'Role counts changed; recheck the approved mapping before applying';
  END IF;
  IF EXISTS (SELECT 1 FROM admin_users WHERE role_code IS NOT NULL AND role_code NOT IN
      ('super_admin','admin','activity_manager','club_manager','member_manager','konselor','asmen','kapro','leaderboard')) THEN
    RAISE EXCEPTION 'Unmapped role exists';
  END IF;
  IF (SELECT count(*) FROM tickets WHERE status = 'open' AND requested_role_code = 'kapro') <> 1
     OR EXISTS (SELECT 1 FROM tickets WHERE status = 'open' AND requested_role_code NOT IN
       ('admin','activity_manager','club_manager','member_manager','konselor','kapro')) THEN
    RAISE EXCEPTION 'Pending requests differ from the approved mapping';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM admin_users WHERE role_code = 'super_admin' AND is_active) THEN
    RAISE EXCEPTION 'An active Super Admin is required';
  END IF;
END $$;

CREATE TEMP TABLE role_change_before ON COMMIT DROP AS
SELECT id, role_code, md5((to_jsonb(a) - 'role_code' - 'updated_at')::text) AS preserved_fields
FROM admin_users a;
CREATE TEMP TABLE ticket_history_before ON COMMIT DROP AS
SELECT id, status = 'open' AND requested_role_code = 'kapro' AS retarget,
  CASE WHEN status = 'open' AND requested_role_code = 'kapro'
    THEN md5((to_jsonb(t) - 'requested_role_code' - 'updated_at')::text)
    ELSE md5(to_jsonb(t)::text)
  END AS preserved_fields
FROM tickets t;

UPDATE admin_users SET role_code = CASE role_code
  WHEN 'asmen' THEN 'admin'
  WHEN 'kapro' THEN 'activity_manager'
  WHEN 'leaderboard' THEN 'member_manager'
END, updated_at = now()
WHERE role_code IN ('asmen','kapro','leaderboard');

-- Explicitly approved: keep this new request pending, with its original reason.
UPDATE tickets SET requested_role_code = 'activity_manager', updated_at = now()
WHERE status = 'open' AND requested_role_code = 'kapro';

DO $$
BEGIN
  IF (SELECT count(*) FROM admin_users) <> (SELECT count(*) FROM role_change_before)
     OR EXISTS (SELECT 1 FROM admin_users a JOIN role_change_before b USING(id)
       WHERE md5((to_jsonb(a) - 'role_code' - 'updated_at')::text) <> b.preserved_fields) THEN
    RAISE EXCEPTION 'Account identity, credentials, status, or count changed';
  END IF;
  IF (SELECT count(*) FROM admin_users a JOIN role_change_before b USING(id)
      WHERE a.role_code IS DISTINCT FROM b.role_code) <> 5 THEN
    RAISE EXCEPTION 'Unexpected number of role changes';
  END IF;
  IF (SELECT count(*) FROM tickets) <> (SELECT count(*) FROM ticket_history_before)
     OR EXISTS (SELECT 1 FROM tickets t JOIN ticket_history_before b USING(id)
       WHERE CASE WHEN b.retarget
         THEN md5((to_jsonb(t) - 'requested_role_code' - 'updated_at')::text)
         ELSE md5(to_jsonb(t)::text) END <> b.preserved_fields
       OR (b.retarget AND t.requested_role_code <> 'activity_manager')) THEN
    RAISE EXCEPTION 'Request history changed';
  END IF;
END $$;

SELECT b.role_code AS previous_role, a.role_code AS current_role, count(*)::integer AS accounts
FROM admin_users a JOIN role_change_before b USING(id)
WHERE a.role_code IS DISTINCT FROM b.role_code
GROUP BY b.role_code, a.role_code ORDER BY b.role_code;
SELECT count(*)::integer AS pending_requests_retargeted FROM ticket_history_before WHERE retarget;
COMMIT;
