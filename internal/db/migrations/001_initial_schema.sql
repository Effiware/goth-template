-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ### Users (mirrored from the identity provider)
-- email is nullable by design: user_name is the canonical textual identifier.
CREATE TABLE users (
    id              UUID PRIMARY KEY,
    user_name       TEXT NOT NULL,
    email           TEXT,
    enabled         BOOLEAN NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    groups          TEXT[],
    attributes      JSONB,
    created_at      TIMESTAMPTZ NOT NULL,
    last_active_at  TIMESTAMPTZ,
    modified_at     TIMESTAMPTZ,
    modified_by     TEXT
);
CREATE INDEX idx_users_on_user_name ON users(user_name);
CREATE INDEX idx_users_name_trgm ON users USING GIN (
    (COALESCE(first_name, '') || ' ' || COALESCE(last_name, '')) gin_trgm_ops
);
CREATE INDEX idx_users_user_name_trgm ON users USING GIN (user_name gin_trgm_ops);


-- ### Organizations (tenant scope; mirrored from the identity provider)
CREATE TABLE organizations (
    id              UUID PRIMARY KEY,
    name            TEXT NOT NULL,
    display_name    TEXT,
    url             TEXT,
    domains         TEXT[],
    attributes      JSONB,
    realm_id        UUID NOT NULL,
    realm_name      TEXT NOT NULL,
    last_event_at   TIMESTAMPTZ,  -- webhook watermark; modified_at can't order events
    created_at      TIMESTAMPTZ NOT NULL,
    modified_at     TIMESTAMPTZ,
    modified_by     TEXT
);
CREATE INDEX idx_organization_on_realm ON organizations(realm_name);

--- # Organization Members
CREATE TABLE organization_members (
    user_id             UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id     UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    organization_roles  TEXT[] NOT NULL DEFAULT '{}',
    created_at          TIMESTAMPTZ NOT NULL,
    modified_at         TIMESTAMPTZ,
    modified_by         TEXT,
    PRIMARY KEY (user_id, organization_id)
);
CREATE INDEX idx_organization_members_on_user ON organization_members(user_id);
CREATE INDEX idx_organization_members_on_org ON organization_members(organization_id);


-- ### Webhooks (inbound provider callbacks, one row per registration)
CREATE TYPE webhook_type AS ENUM ('generic', 'other');
CREATE TABLE tech_webhooks (
    name                VARCHAR(32) PRIMARY KEY,
    external_id         TEXT NOT NULL UNIQUE,    -- provider-assigned ID
    url                 TEXT NOT NULL,
    type                webhook_type NOT NULL,
    enabled             BOOLEAN NOT NULL,
    details             JSONB,
    last_synced_at      TIMESTAMPTZ NOT NULL,
    last_operation_type TEXT,
    last_used_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL,
    created_by          TEXT NOT NULL            -- no history table for this one
);

-- ### Service runs (background Servicer heartbeat / last-run state; see internal/services)
CREATE TABLE tech_service_runs (
    service_name     TEXT PRIMARY KEY,
    last_started_at  TIMESTAMPTZ NOT NULL,
    last_finished_at TIMESTAMPTZ,
    last_status      TEXT NOT NULL,                -- 'ok' | 'error'
    last_error       TEXT,
    duration_ms      BIGINT,
    details          JSONB NOT NULL DEFAULT '{}'   -- per-run counts
);


-- ### Teams
CREATE TYPE team_type AS ENUM ('regular', 'custom');
CREATE TABLE teams (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    type            team_type NOT NULL DEFAULT 'regular',
    name            VARCHAR(52) NOT NULL,
    description     TEXT,
    tags            TEXT[],
    primary_color   TEXT NOT NULL DEFAULT 'blue',
    secondary_color TEXT NOT NULL DEFAULT 'blue',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at     TIMESTAMPTZ,
    modified_by     TEXT,
    UNIQUE(organization_id, name),
    CHECK(length(name) >= 3)
);
CREATE INDEX idx_team_on_tags_gin  ON teams USING GIN (tags);
CREATE INDEX idx_team_on_name_trgm ON teams USING GIN (name gin_trgm_ops);

--- # Team Members
CREATE TYPE team_member_type AS ENUM ('owner', 'member', 'viewer');
CREATE TABLE team_members (
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    team_id         UUID NOT NULL REFERENCES teams(id) ON DELETE CASCADE,
    member_type     team_member_type NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    modified_at     TIMESTAMPTZ,
    modified_by     TEXT,
    PRIMARY KEY (user_id, team_id)
);
CREATE INDEX idx_team_members_on_team ON team_members(team_id);
CREATE INDEX idx_team_members_on_user ON team_members(user_id);

-- +goose Down
DROP TABLE team_members;
DROP TABLE teams;
DROP TABLE tech_service_runs;
DROP TABLE tech_webhooks;
DROP TABLE organization_members;
DROP TABLE organizations;
DROP TABLE users;
DROP TYPE team_member_type;
DROP TYPE team_type;
DROP TYPE webhook_type;
DROP EXTENSION pg_trgm;
DROP EXTENSION "uuid-ossp";
