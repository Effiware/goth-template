-- name: UpsertServiceRun :exec
INSERT INTO tech_service_runs (
    service_name, last_started_at, last_finished_at, last_status, last_error, duration_ms, details
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) ON CONFLICT (service_name) DO UPDATE SET
    last_started_at  = excluded.last_started_at,
    last_finished_at = excluded.last_finished_at,
    last_status      = excluded.last_status,
    last_error       = excluded.last_error,
    duration_ms      = excluded.duration_ms,
    details          = excluded.details;

-- name: ListServiceRuns :many
SELECT * FROM tech_service_runs
ORDER BY service_name;

-- name: IsServiceRunFresh :one
-- Gate check for services.RunIfDue: true when the named service last started
-- within the freshness window. now() is evaluated DB-side, which takes the
-- reader's clock out of the comparison; last_started_at is still stamped from
-- the writing instance's clock (RecordRun), so cross-instance skew eats into
-- the freshness slack — acceptable with NTP-synced hosts, where skew is far
-- below the 10% slack. last_started_at (not finished) keeps the effective
-- cadence start-to-start.
SELECT EXISTS (
    SELECT 1 FROM tech_service_runs
    WHERE service_name = sqlc.arg(service_name)
      AND last_started_at > now() - make_interval(secs => sqlc.arg(freshness_seconds)::float8)
) AS fresh;
