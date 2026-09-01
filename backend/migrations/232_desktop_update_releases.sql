CREATE TABLE IF NOT EXISTS desktop_update_releases (
    id BIGSERIAL PRIMARY KEY,
    public_id VARCHAR(40) NOT NULL UNIQUE,
    version VARCHAR(64) NOT NULL UNIQUE,
    notes TEXT NOT NULL DEFAULT '',
    artifacts JSONB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    created_by BIGINT,
    updated_by BIGINT,
    published_by BIGINT,
    withdrawn_by BIGINT,
    published_at TIMESTAMPTZ,
    withdrawn_at TIMESTAMPTZ,
    withdrawal_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT desktop_update_releases_status_check
        CHECK (status IN ('draft', 'published', 'withdrawn')),
    CONSTRAINT desktop_update_releases_artifacts_object_check
        CHECK (jsonb_typeof(artifacts) = 'object'),
    CONSTRAINT desktop_update_releases_artifacts_platforms_check CHECK (
        jsonb_object_length(artifacts) = 3
        AND artifacts ?& ARRAY['darwin-aarch64', 'darwin-x86_64', 'windows-x86_64']
    ),
    CONSTRAINT desktop_update_releases_artifacts_shape_check CHECK (
        jsonb_typeof(artifacts #> '{darwin-aarch64}') = 'object'
        AND jsonb_typeof(artifacts #> '{darwin-aarch64,url}') = 'string'
        AND jsonb_typeof(artifacts #> '{darwin-aarch64,signature}') = 'string'
		AND jsonb_typeof(artifacts #> '{darwin-aarch64,object_key}') = 'string'
		AND jsonb_typeof(artifacts #> '{darwin-aarch64,file_name}') = 'string'
		AND jsonb_typeof(artifacts #> '{darwin-aarch64,size}') = 'number'
		AND jsonb_typeof(artifacts #> '{darwin-aarch64,sha256}') = 'string'
        AND jsonb_typeof(artifacts #> '{darwin-x86_64}') = 'object'
        AND jsonb_typeof(artifacts #> '{darwin-x86_64,url}') = 'string'
        AND jsonb_typeof(artifacts #> '{darwin-x86_64,signature}') = 'string'
		AND jsonb_typeof(artifacts #> '{darwin-x86_64,object_key}') = 'string'
		AND jsonb_typeof(artifacts #> '{darwin-x86_64,file_name}') = 'string'
		AND jsonb_typeof(artifacts #> '{darwin-x86_64,size}') = 'number'
		AND jsonb_typeof(artifacts #> '{darwin-x86_64,sha256}') = 'string'
        AND jsonb_typeof(artifacts #> '{windows-x86_64}') = 'object'
        AND jsonb_typeof(artifacts #> '{windows-x86_64,url}') = 'string'
        AND jsonb_typeof(artifacts #> '{windows-x86_64,signature}') = 'string'
		AND jsonb_typeof(artifacts #> '{windows-x86_64,object_key}') = 'string'
		AND jsonb_typeof(artifacts #> '{windows-x86_64,file_name}') = 'string'
		AND jsonb_typeof(artifacts #> '{windows-x86_64,size}') = 'number'
		AND jsonb_typeof(artifacts #> '{windows-x86_64,sha256}') = 'string'
    ),
    CONSTRAINT desktop_update_releases_lifecycle_check CHECK (
        (status = 'draft' AND published_at IS NULL AND withdrawn_at IS NULL AND withdrawal_reason IS NULL)
        OR (status = 'published' AND published_at IS NOT NULL AND withdrawn_at IS NULL AND withdrawal_reason IS NULL)
        OR (status = 'withdrawn' AND published_at IS NOT NULL AND withdrawn_at IS NOT NULL AND withdrawal_reason IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_desktop_update_releases_status_published
    ON desktop_update_releases(status, published_at DESC);
CREATE INDEX IF NOT EXISTS idx_desktop_update_releases_created_at
    ON desktop_update_releases(created_at DESC);
