-- Expand first. Existing memberships remain unknown until explicitly assigned.
CREATE TABLE IF NOT EXISTS desktop_departments (
 id BIGSERIAL PRIMARY KEY, public_id VARCHAR(40) NOT NULL UNIQUE,
 organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id) ON DELETE RESTRICT,
 name VARCHAR(100) NOT NULL, name_normalized VARCHAR(400) NOT NULL,
 status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CONSTRAINT idx_desktop_departments_name UNIQUE(organization_id,name_normalized),
 CONSTRAINT idx_desktop_departments_org_id UNIQUE(organization_id,id)
);
CREATE TABLE IF NOT EXISTS desktop_positions (
 id BIGSERIAL PRIMARY KEY, public_id VARCHAR(40) NOT NULL UNIQUE,
 organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id) ON DELETE RESTRICT,
 department_id BIGINT NOT NULL REFERENCES desktop_departments(id) ON DELETE RESTRICT,
 name VARCHAR(100) NOT NULL, name_normalized VARCHAR(400) NOT NULL,
 status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
 revision BIGINT NOT NULL DEFAULT 1 CHECK(revision>0),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CONSTRAINT idx_desktop_positions_name UNIQUE(organization_id,department_id,name_normalized),
 CONSTRAINT idx_desktop_positions_org_id UNIQUE(organization_id,id),
 FOREIGN KEY(organization_id,department_id) REFERENCES desktop_departments(organization_id,id) ON DELETE RESTRICT
);
ALTER TABLE desktop_members ADD COLUMN IF NOT EXISTS position_id BIGINT REFERENCES desktop_positions(id) ON DELETE RESTRICT;
ALTER TABLE desktop_members ADD COLUMN IF NOT EXISTS position_revision BIGINT NOT NULL DEFAULT 0 CHECK(position_revision>=0);
CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_members_org_id ON desktop_members(organization_id,id);
CREATE INDEX IF NOT EXISTS idx_desktop_members_position ON desktop_members(organization_id,position_id,id);
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_constraint WHERE conname='desktop_members_position_organization_fkey' AND conrelid='desktop_members'::regclass) THEN
  ALTER TABLE desktop_members ADD CONSTRAINT desktop_members_position_organization_fkey FOREIGN KEY(organization_id,position_id) REFERENCES desktop_positions(organization_id,id) ON DELETE RESTRICT;
 END IF;
END $$;
CREATE TABLE IF NOT EXISTS desktop_member_position_history (
 id BIGSERIAL PRIMARY KEY, organization_id BIGINT NOT NULL,
 member_id BIGINT NOT NULL, position_id BIGINT NOT NULL,
 position_revision BIGINT NOT NULL CHECK(position_revision>0),
 department_name_snapshot VARCHAR(100) NOT NULL, position_name_snapshot VARCHAR(100) NOT NULL,
 started_at TIMESTAMPTZ NOT NULL, ended_at TIMESTAMPTZ,
 source VARCHAR(32) NOT NULL CHECK(source IN ('member_create','transfer','migration_confirmed')),
 changed_by_user_id BIGINT, request_id VARCHAR(128) NOT NULL DEFAULT '', reason VARCHAR(500) NOT NULL DEFAULT '',
 FOREIGN KEY(organization_id,member_id) REFERENCES desktop_members(organization_id,id) ON DELETE RESTRICT,
 FOREIGN KEY(organization_id,position_id) REFERENCES desktop_positions(organization_id,id) ON DELETE RESTRICT,
 CHECK(ended_at IS NULL OR ended_at>started_at)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_position_history_revision ON desktop_member_position_history(member_id,position_revision);
CREATE UNIQUE INDEX IF NOT EXISTS idx_desktop_position_history_current ON desktop_member_position_history(member_id) WHERE ended_at IS NULL;
