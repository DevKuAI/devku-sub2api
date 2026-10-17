-- Existing invitations keep working through their original token.
-- A separate hashed credential fits WeChat's 32-character scene limit.
ALTER TABLE bi_invitations ADD COLUMN IF NOT EXISTS scene_hash VARCHAR(64);
CREATE UNIQUE INDEX IF NOT EXISTS bi_invitations_scene_hash_unique
    ON bi_invitations (scene_hash) WHERE scene_hash IS NOT NULL;
