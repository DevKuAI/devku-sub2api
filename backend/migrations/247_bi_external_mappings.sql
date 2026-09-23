CREATE TABLE IF NOT EXISTS bi_external_mappings (
    organization_id VARCHAR(128) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    entity_kind VARCHAR(32) NOT NULL,
    external_id TEXT NOT NULL,
    entity_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_id,source_id,entity_kind,external_id),
    UNIQUE (organization_id,entity_kind,entity_id),
    FOREIGN KEY (organization_id,source_id) REFERENCES bi_connector_sources(organization_id,source_id)
);
