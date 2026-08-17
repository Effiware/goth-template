package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// HistoryKind identifies a single-UUID-keyed history entity for SnapshotEntity.
type HistoryKind string

const (
	HistoryOrganization HistoryKind = "organization"
	HistoryTeam         HistoryKind = "team"
)

// SystemActor is the changed_by value for background/service-driven snapshots.
const SystemActor = "system"

// SnapshotEntity writes an after-image history snapshot, self-computing its
// version (MAX+1). Call it inside the SAME transaction as the base-row mutation —
// the mutation's row lock makes the version read race-safe. note may be "".
func SnapshotEntity(ctx context.Context, q Querier, kind HistoryKind, id pgtype.UUID, actor, note string) error {
	changeNote := pgtype.Text{String: note, Valid: note != ""}
	switch kind {
	case HistoryOrganization:
		return q.SnapshotOrganizationHistory(ctx, SnapshotOrganizationHistoryParams{ID: id, ChangedBy: actor, ChangeNote: changeNote})
	case HistoryTeam:
		return q.SnapshotTeamHistory(ctx, SnapshotTeamHistoryParams{ID: id, ChangedBy: actor, ChangeNote: changeNote})
	default:
		return fmt.Errorf("SnapshotEntity: unknown history kind %q", kind)
	}
}
