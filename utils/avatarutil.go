package utils

import "github.com/jackc/pgx/v5/pgtype"

// avatarPalette excludes amber, emerald, rose and sky: those hues already encode
// assignment status and team role elsewhere, and identity must not collide with them.
var avatarPalette = [8]string{
	"bg-violet-100 text-violet-700",
	"bg-blue-100 text-blue-700",
	"bg-indigo-100 text-indigo-700",
	"bg-fuchsia-100 text-fuchsia-700",
	"bg-pink-100 text-pink-700",
	"bg-teal-100 text-teal-700",
	"bg-cyan-100 text-cyan-700",
	"bg-lime-100 text-lime-700",
}

// AvatarColorClasses picks a swatch from the UUID's first byte, so a user reads the
// same on every surface. Invalid UUIDs fall back to the first entry.
func AvatarColorClasses(id pgtype.UUID) string {
	if !id.Valid {
		return avatarPalette[0]
	}
	return avatarPalette[int(id.Bytes[0])%len(avatarPalette)]
}

// AvatarColorClassesFromSub is the string-keyed variant (a Keycloak subject),
// agreeing with AvatarColorClasses for the same id.
func AvatarColorClassesFromSub(sub string) string {
	id, err := ParseUUID(sub)
	if err != nil {
		return avatarPalette[0]
	}
	return AvatarColorClasses(id)
}
