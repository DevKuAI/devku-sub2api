-- Conversation reporting requires an explicit administrator opt-in per organization.
ALTER TABLE desktop_organizations
    ADD COLUMN IF NOT EXISTS conversation_reporting_enabled BOOLEAN NOT NULL DEFAULT FALSE;
