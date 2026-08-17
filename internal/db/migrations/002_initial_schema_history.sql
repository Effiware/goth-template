-- +goose Up

-- ### History / audit-trail tables for the 001 domain.
--
-- After-image full snapshot: one row per version, mirroring the base row's state
-- columns plus version + changed_at/changed_by/change_note. v1 = create; each
-- snapshot self-computes version as MAX(version)+1 at insert time. Written
-- app-level inside the same tx as the mutation (see db.SnapshotEntity), never by
-- trigger — the mutation already row-locks the base row, so MAX+1 is race-safe.
--
-- No foreign keys here on purpose: history never blocks a delete, and the trail
-- survives the row it describes. Growth is unbounded; retention is deferred.

CREATE TABLE organizations_history (
    id           UUID NOT NULL,
    version      INTEGER NOT NULL,
    name         TEXT NOT NULL,
    display_name TEXT,
    url          TEXT,
    domains      TEXT[],
    attributes   JSONB,
    realm_id     UUID NOT NULL,
    realm_name   TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,  -- base-row lifecycle, carried through
    modified_at  TIMESTAMPTZ,
    modified_by  TEXT,

    changed_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    changed_by   TEXT NOT NULL,         -- user id, or 'system'
    change_note  TEXT,

    PRIMARY KEY (id, version)
);

CREATE TABLE teams_history (
    id              UUID NOT NULL,
    version         INTEGER NOT NULL,
    organization_id UUID NOT NULL,      -- denormalized scope key
    type            team_type NOT NULL,
    name            VARCHAR(52) NOT NULL,
    description     TEXT,
    tags            TEXT[],
    primary_color   TEXT NOT NULL,
    secondary_color TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL,
    modified_at     TIMESTAMPTZ,
    modified_by     TEXT,

    changed_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    changed_by      TEXT NOT NULL,
    change_note     TEXT,

    PRIMARY KEY (id, version)
);
CREATE INDEX idx_teams_history_on_org ON teams_history (organization_id, changed_at);

-- +goose Down
DROP TABLE teams_history;
DROP TABLE organizations_history;
