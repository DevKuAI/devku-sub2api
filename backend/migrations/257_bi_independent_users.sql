-- Retire unpublished BI development data. Desktop sources and settings are retained.
DROP TABLE IF EXISTS bi_acl_denials;
DROP TABLE IF EXISTS bi_analysis_contexts;
DROP TABLE IF EXISTS bi_binding_challenges;
DROP TABLE IF EXISTS bi_cleanup_runs;
DROP TABLE IF EXISTS bi_command_receipts;
DROP TABLE IF EXISTS bi_connector_credentials;
DROP TABLE IF EXISTS bi_entity_versions;
DROP TABLE IF EXISTS bi_external_mappings;
DROP TABLE IF EXISTS bi_favorites;
DROP TABLE IF EXISTS bi_ingestion_outbox;
DROP TABLE IF EXISTS bi_knowledge_notes;
DROP TABLE IF EXISTS bi_list_snapshots;
DROP TABLE IF EXISTS bi_manager_grants;
DROP TABLE IF EXISTS bi_record_receipts;
DROP TABLE IF EXISTS bi_refresh_tokens;
DROP TABLE IF EXISTS bi_report_shares;
DROP TABLE IF EXISTS bi_retention_policies;
DROP TABLE IF EXISTS bi_security_events;
DROP TABLE IF EXISTS bi_source_version_links;
DROP TABLE IF EXISTS bi_usage_daily;
DROP TABLE IF EXISTS bi_usage_facts;
DROP TABLE IF EXISTS bi_wechat_codes;
DROP TABLE IF EXISTS bi_entities;
DROP TABLE IF EXISTS bi_reports;
DROP TABLE IF EXISTS bi_sessions;
DROP TABLE IF EXISTS bi_usage_day_revisions;
DROP TABLE IF EXISTS bi_data_revisions;
DROP TABLE IF EXISTS bi_wechat_bindings;
DROP TABLE IF EXISTS bi_import_batches;
DROP TABLE IF EXISTS bi_managers;
DROP TABLE IF EXISTS bi_connector_sources;
DROP TABLE IF EXISTS bi_organizations;

CREATE TABLE bi_users (
 id VARCHAR(128) PRIMARY KEY,
 appid VARCHAR(128) NOT NULL,
 openid_hash CHAR(64) NOT NULL,
 display_name VARCHAR(80) NOT NULL DEFAULT '小程序使用者',
 status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
 auth_version BIGINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 UNIQUE(appid,openid_hash)
);
CREATE TABLE bi_access (
 bi_user_id VARCHAR(128) NOT NULL REFERENCES bi_users(id),
 organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 capabilities JSONB NOT NULL CHECK(jsonb_typeof(capabilities)='array'),
 status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK(status IN ('active','revoked')),
 revision BIGINT NOT NULL DEFAULT 1,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 PRIMARY KEY(bi_user_id,organization_id)
);
CREATE TABLE bi_invitations (
 id VARCHAR(128) PRIMARY KEY,
 organization_id BIGINT NOT NULL REFERENCES desktop_organizations(id),
 token_hash CHAR(64) NOT NULL UNIQUE,
 capabilities JSONB NOT NULL CHECK(jsonb_typeof(capabilities)='array'),
 note VARCHAR(100) NOT NULL DEFAULT '',
 status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','used','revoked')),
 claimed_by VARCHAR(128) REFERENCES bi_users(id),
 claimed_at TIMESTAMPTZ,
 expires_at TIMESTAMPTZ NOT NULL,
 created_by BIGINT NOT NULL REFERENCES users(id),
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 CHECK((status='used')=(claimed_by IS NOT NULL))
);
CREATE INDEX bi_invitation_organization ON bi_invitations(organization_id,created_at DESC);
CREATE TABLE bi_sessions (
 id VARCHAR(128) PRIMARY KEY,
 bi_user_id VARCHAR(128) NOT NULL REFERENCES bi_users(id),
 auth_version BIGINT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 revoked_at TIMESTAMPTZ,
 last_seen_at TIMESTAMPTZ NOT NULL,
 client_version VARCHAR(32) NOT NULL DEFAULT ''
);
CREATE INDEX bi_session_user ON bi_sessions(bi_user_id);
CREATE TABLE bi_refresh_tokens (
 token_hash CHAR(64) PRIMARY KEY,
 session_id VARCHAR(128) NOT NULL REFERENCES bi_sessions(id) ON DELETE CASCADE,
 consumed_at TIMESTAMPTZ,
 expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE bi_wechat_codes (code_hash CHAR(64) PRIMARY KEY,created_at TIMESTAMPTZ NOT NULL);
CREATE TABLE bi_security_events (
 id BIGSERIAL PRIMARY KEY,
 actor_type VARCHAR(16) NOT NULL CHECK(actor_type IN ('web_user','bi_user','system')),
 actor_id VARCHAR(128) NOT NULL,
 organization_id BIGINT REFERENCES desktop_organizations(id),
 action VARCHAR(64) NOT NULL,
 target_id VARCHAR(128) NOT NULL,
 request_id VARCHAR(128) NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX bi_audit_organization ON bi_security_events(organization_id,created_at DESC);
CREATE INDEX bi_audit_request ON bi_security_events(request_id);
CREATE TABLE bi_retention_policies (
 id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK(id),
 audit_days INTEGER NOT NULL DEFAULT 180 CHECK(audit_days BETWEEN 1 AND 3650),
 ephemeral_days INTEGER NOT NULL DEFAULT 7 CHECK(ephemeral_days BETWEEN 1 AND 365)
);
INSERT INTO bi_retention_policies(id) VALUES(TRUE);
CREATE TABLE bi_cleanup_runs (
 id BIGSERIAL PRIMARY KEY,
 finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
 deleted_count BIGINT NOT NULL,
 status VARCHAR(16) NOT NULL CHECK(status IN ('succeeded','failed'))
);
