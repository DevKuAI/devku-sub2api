CREATE TABLE IF NOT EXISTS bi_usage_day_revisions (
    organization_id VARCHAR(128) NOT NULL REFERENCES bi_organizations(id),
    day DATE NOT NULL,
    data_revision BIGINT NOT NULL REFERENCES bi_data_revisions(id),
    PRIMARY KEY (organization_id,day,data_revision)
);

CREATE TABLE IF NOT EXISTS bi_usage_daily (
    organization_id VARCHAR(128) NOT NULL,
    day DATE NOT NULL,
    data_revision BIGINT NOT NULL,
    dimension_hash CHAR(64) NOT NULL,
    source_id VARCHAR(128) NOT NULL,
    team_id VARCHAR(128),
    actor_type VARCHAR(16) NOT NULL,
    application_id VARCHAR(128),
    scene_id VARCHAR(128),
    requested_model TEXT NOT NULL,
    requests BIGINT NOT NULL CHECK (requests > 0),
    succeeded BIGINT NOT NULL CHECK (succeeded >= 0),
    failed BIGINT NOT NULL CHECK (failed >= 0),
    unknown_outcome BIGINT NOT NULL CHECK (unknown_outcome >= 0),
    measured BIGINT NOT NULL CHECK (measured >= 0),
    unmeasured BIGINT NOT NULL CHECK (unmeasured >= 0),
    input_tokens NUMERIC,
    output_tokens NUMERIC,
    cache_read_tokens NUMERIC,
    cache_write_tokens NUMERIC,
    PRIMARY KEY (organization_id,day,data_revision,dimension_hash),
    FOREIGN KEY (organization_id,day,data_revision) REFERENCES bi_usage_day_revisions(organization_id,day,data_revision),
    CHECK (requests = succeeded + failed + unknown_outcome),
    CHECK (requests = measured + unmeasured)
);
CREATE INDEX IF NOT EXISTS bi_usage_daily_scope ON bi_usage_daily(organization_id,day,team_id,application_id,scene_id);
