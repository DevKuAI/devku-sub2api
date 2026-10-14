ALTER TABLE desktop_update_releases ADD COLUMN rollout_mode varchar(20) NOT NULL DEFAULT 'full', ADD COLUMN gateway_user_ids jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE desktop_update_releases ADD CONSTRAINT desktop_update_rollout_mode_check CHECK (rollout_mode IN ('full', 'gray'));
