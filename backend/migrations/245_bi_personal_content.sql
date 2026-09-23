CREATE TABLE IF NOT EXISTS bi_favorites (
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    knowledge_id VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (manager_id,organization_id,knowledge_id)
);
CREATE TABLE IF NOT EXISTS bi_knowledge_notes (
    manager_id VARCHAR(128) NOT NULL REFERENCES bi_managers(id),
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    knowledge_id VARCHAR(128) NOT NULL,
    text TEXT NOT NULL CHECK (char_length(text) BETWEEN 1 AND 300),
    revision VARCHAR(128) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (manager_id,organization_id,knowledge_id)
);
