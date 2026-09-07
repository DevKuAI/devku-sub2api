ALTER TABLE desktop_organizations
    ADD COLUMN IF NOT EXISTS member_limit INTEGER;

-- Preserve existing membership and explicitly configured carrier capacity.
UPDATE desktop_organizations AS organization
SET member_limit = GREATEST(
    10,
    (SELECT api_key_limit FROM users WHERE id = organization.gateway_user_id),
    (SELECT COUNT(*)::INTEGER FROM desktop_members
     WHERE organization_id = organization.id AND deleted_at IS NULL)
)
WHERE member_limit IS NULL;

ALTER TABLE desktop_organizations
    ALTER COLUMN member_limit SET DEFAULT 10,
    ALTER COLUMN member_limit SET NOT NULL,
    ADD CONSTRAINT desktop_organizations_member_limit_positive CHECK (member_limit > 0);

UPDATE users AS carrier
SET api_key_limit = organization.member_limit, updated_at = NOW()
FROM desktop_organizations AS organization
WHERE carrier.id = organization.gateway_user_id
  AND carrier.deleted_at IS NULL
  AND organization.deleted_at IS NULL
  AND carrier.api_key_limit IS DISTINCT FROM organization.member_limit;
