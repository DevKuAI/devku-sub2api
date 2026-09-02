-- Preserve Desktop carrier access for organizations whose group is already exclusive.
INSERT INTO user_allowed_groups (user_id, group_id)
SELECT DISTINCT organization.gateway_user_id, organization.group_id
FROM desktop_organizations AS organization
JOIN groups AS group_record ON group_record.id = organization.group_id
WHERE organization.deleted_at IS NULL
  AND group_record.is_exclusive = TRUE
ON CONFLICT (user_id, group_id) DO NOTHING;
