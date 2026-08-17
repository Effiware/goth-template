-- Audit-trail history writes. Every query here is called app-level inside the
-- same tx as the base-row mutation (see SnapshotEntity in history.go); the
-- version is self-computed as MAX+1, race-safe under the mutation's row lock.

-- name: SnapshotOrganizationHistory :exec
INSERT INTO organizations_history (
    id, name, display_name, url, domains, attributes, realm_id, realm_name,
    created_at, modified_at, modified_by,
    version, changed_by, change_note
)
SELECT
    o.id, o.name, o.display_name, o.url, o.domains, o.attributes, o.realm_id, o.realm_name,
    o.created_at, o.modified_at, o.modified_by,
    (SELECT COALESCE(MAX(h.version), 0) + 1 FROM organizations_history h WHERE h.id = o.id),
    sqlc.arg(changed_by), sqlc.arg(change_note)
FROM organizations o
WHERE o.id = sqlc.arg(id);

-- name: SnapshotTeamHistory :exec
INSERT INTO teams_history (
    id, organization_id, type, name, description, tags, primary_color, secondary_color,
    created_at, modified_at, modified_by,
    version, changed_by, change_note
)
SELECT
    t.id, t.organization_id, t.type, t.name, t.description, t.tags, t.primary_color, t.secondary_color,
    t.created_at, t.modified_at, t.modified_by,
    (SELECT COALESCE(MAX(h.version), 0) + 1 FROM teams_history h WHERE h.id = t.id),
    sqlc.arg(changed_by), sqlc.arg(change_note)
FROM teams t
WHERE t.id = sqlc.arg(id);
