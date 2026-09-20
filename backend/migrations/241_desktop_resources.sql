CREATE TABLE desktop_resources (
    id VARCHAR(36) PRIMARY KEY CHECK (id ~ '^res_[0-9a-f]{32}$'),
    resource_key VARCHAR(64) NOT NULL UNIQUE,
    kind VARCHAR(10) NOT NULL CHECK (kind IN ('mcp', 'skill')),
    current_version VARCHAR(20) NOT NULL,
    status VARCHAR(10) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    status_reason TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL,
    updated_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (status <> 'disabled' OR length(status_reason) > 0)
);

CREATE TABLE desktop_resource_versions (
    resource_id VARCHAR(36) NOT NULL REFERENCES desktop_resources(id),
    version VARCHAR(20) NOT NULL CHECK (version ~ '^(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})\.(0|[1-9][0-9]{0,5})$'),
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    source_url TEXT NOT NULL,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (resource_id, version)
);

ALTER TABLE desktop_resources ADD CONSTRAINT desktop_resource_current_version_fk
    FOREIGN KEY (id, current_version) REFERENCES desktop_resource_versions(resource_id, version)
    DEFERRABLE INITIALLY DEFERRED;

CREATE TABLE desktop_resource_artifacts (
    resource_id VARCHAR(36) NOT NULL,
    version VARCHAR(20) NOT NULL,
    platform VARCHAR(20) NOT NULL CHECK (platform IN ('darwin-arm64', 'darwin-x64', 'windows-x64', 'linux-x64', 'any')),
    sha256 VARCHAR(64) NOT NULL CHECK (sha256 ~ '^[0-9a-f]{64}$'),
    size_bytes BIGINT NOT NULL CHECK (size_bytes > 0 AND size_bytes <= 67108864),
    manifest JSONB NOT NULL CHECK (jsonb_typeof(manifest) = 'object'),
    object_key TEXT NOT NULL UNIQUE,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (resource_id, version, platform),
    FOREIGN KEY (resource_id, version) REFERENCES desktop_resource_versions(resource_id, version)
);
CREATE INDEX desktop_resources_listing_idx ON desktop_resources (status, updated_at DESC, id);
