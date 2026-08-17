package utils

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// ParseUUID parses a UUID string into a pgtype.UUID.
func ParseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("failed to parse UUID %q: %w", s, err)
	}
	return u, nil
}

// UUIDString renders a pgtype.UUID hyphenated, "" when invalid.
func UUIDString(id pgtype.UUID) string {
	if !id.Valid {
		return ""
	}
	b := id.Bytes
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
