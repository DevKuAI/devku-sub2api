-- Keep organization labels separate from the independent WeChat identity.
ALTER TABLE bi_access ADD COLUMN IF NOT EXISTS organization_alias VARCHAR(80) NOT NULL DEFAULT '';
ALTER TABLE bi_invitations ADD COLUMN IF NOT EXISTS recipient_name VARCHAR(80) NOT NULL DEFAULT '';
