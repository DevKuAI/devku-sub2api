-- Run with psql --single-transaction only after every enterprise has been mapped.
-- Kept outside the embedded migration glob to avoid enforcing unknown legacy assignments.
-- Drain application writers at their first enterprise lock before locking child tables.
LOCK TABLE desktop_organizations IN EXCLUSIVE MODE;
LOCK TABLE desktop_members, desktop_positions, desktop_departments, desktop_member_position_history IN SHARE ROW EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS (
  SELECT 1 FROM desktop_members m WHERE m.deleted_at IS NULL AND (
   m.position_id IS NULL OR NOT EXISTS (
    SELECT 1 FROM desktop_member_position_history h
    WHERE h.member_id=m.id AND h.organization_id=m.organization_id
     AND h.position_id=m.position_id AND h.position_revision=m.position_revision AND h.ended_at IS NULL
   )
  )
 ) THEN
  RAISE EXCEPTION 'Desktop organization assignments are incomplete or inconsistent; inspect organization-structure/status before enforcing';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='desktop_members_position_required' AND conrelid='desktop_members'::regclass) THEN
  ALTER TABLE desktop_members ADD CONSTRAINT desktop_members_position_required CHECK(deleted_at IS NOT NULL OR position_id IS NOT NULL) NOT VALID;
 END IF;
END $$;
ALTER TABLE desktop_members VALIDATE CONSTRAINT desktop_members_position_required;
