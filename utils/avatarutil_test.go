package utils

import (
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func uuidWithFirstByte(b byte) pgtype.UUID {
	var u pgtype.UUID
	u.Valid = true
	u.Bytes[0] = b
	return u
}

func TestAvatarColorClasses_Stability(t *testing.T) {
	id := uuidWithFirstByte(0x42)
	first := AvatarColorClasses(id)
	for i := 0; i < 10; i++ {
		if got := AvatarColorClasses(id); got != first {
			t.Fatalf("non-deterministic palette: iteration %d got %q, expected %q", i, got, first)
		}
	}
}

func TestAvatarColorClasses_InvalidUUIDFallback(t *testing.T) {
	var invalid pgtype.UUID
	if got := AvatarColorClasses(invalid); got != avatarPalette[0] {
		t.Errorf("invalid UUID should fall back to palette[0], got %q", got)
	}
}

func TestAvatarColorClasses_AllPaletteEntriesReachable(t *testing.T) {
	seen := make(map[string]bool, len(avatarPalette))
	for b := 0; b < 256; b++ {
		seen[AvatarColorClasses(uuidWithFirstByte(byte(b)))] = true
	}
	if len(seen) != len(avatarPalette) {
		t.Errorf("expected all %d palette entries to be reachable, only saw %d", len(avatarPalette), len(seen))
	}
}

func TestAvatarColorClassesFromSub_MatchesUUIDVariant(t *testing.T) {
	sub := "deadbeef-1234-5678-9abc-def012345678"
	id, err := ParseUUID(sub)
	if err != nil {
		t.Fatalf("ParseUUID: %v", err)
	}
	if got, want := AvatarColorClassesFromSub(sub), AvatarColorClasses(id); got != want {
		t.Errorf("FromSub and UUID variants diverged: %q vs %q", got, want)
	}
}

func TestAvatarColorClassesFromSub_ParseFailureFallback(t *testing.T) {
	cases := []string{"", "not-a-uuid", "12345"}
	for _, s := range cases {
		if got := AvatarColorClassesFromSub(s); got != avatarPalette[0] {
			t.Errorf("FromSub(%q) should fall back to palette[0], got %q", s, got)
		}
	}
}
