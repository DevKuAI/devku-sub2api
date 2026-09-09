-- Desktop resources are distributed independently of application installers.
CREATE TABLE desktop_resources (
    id TEXT PRIMARY KEY,
    resource_key TEXT NOT NULL,
    organization_id BIGINT REFERENCES desktop_organizations(id) ON DELETE RESTRICT,
    created_by BIGINT REFERENCES desktop_members(id) ON DELETE SET NULL,
    kind TEXT NOT NULL CHECK (kind IN ('mcp', 'skill')),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    source_url TEXT NOT NULL DEFAULT '',
    current_version TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX desktop_resource_scope_key ON desktop_resources(COALESCE(organization_id, 0), resource_key);
CREATE INDEX desktop_resource_scope_updated ON desktop_resources(organization_id, updated_at DESC, id);
CREATE TABLE desktop_resource_artifacts (
    resource_id TEXT NOT NULL REFERENCES desktop_resources(id) ON DELETE CASCADE,
    version TEXT NOT NULL,
    platform TEXT NOT NULL,
    manifest JSONB NOT NULL,
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 67108864),
    artifact_data BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY(resource_id, version, platform)
);
