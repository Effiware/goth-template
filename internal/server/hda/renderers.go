package hda

import (
	"log/slog"
	"net/http"

	"github.com/effiware/goth-template/internal/clicks"
	"github.com/effiware/goth-template/internal/server/session"
	"github.com/effiware/goth-template/internal/views"
	"github.com/effiware/goth-template/internal/views/components"
)

// sessionClicksKey counts this visitor's own clicks — the demo's proof that the
// session store round-trips. nil sessionStore (no Redis) degrades to 0.
const sessionClicksKey = "clicks"

// RenderRoot renders the full page.
func RenderRoot(counter *clicks.Counter, sessionStore *session.Store, baseURL string) ViewHandlerT {
	return func(w http.ResponseWriter, r *http.Request) error {
		total, err := counter.Count(r.Context())
		if err != nil {
			return err
		}
		return views.Index(baseURL, total, sessionClicks(r, sessionStore)).Render(r.Context(), w)
	}
}

// RenderClick handles the HTMX click: increments both counters, swaps the fragment.
func RenderClick(counter *clicks.Counter, sessionStore *session.Store) ViewHandlerT {
	return func(w http.ResponseWriter, r *http.Request) error {
		total, err := counter.Increment(r.Context())
		if err != nil {
			return err
		}

		mine := sessionClicks(r, sessionStore) + 1
		if sessionStore != nil {
			if err := sessionStore.Set(w, r, map[string]any{sessionClicksKey: mine}); err != nil {
				// The global count is already committed; a lost session write
				// costs this visitor's own tally, nothing more.
				slog.WarnContext(r.Context(), "session: failed to store click count", "error", err)
			}
		}

		return components.Click(total, mine).Render(r.Context(), w)
	}
}

func sessionClicks(r *http.Request, sessionStore *session.Store) int64 {
	if sessionStore == nil {
		return 0
	}
	raw, ok := sessionStore.Value(r, sessionClicksKey)
	if !ok {
		return 0
	}
	count, _ := raw.(int64)
	return count
}
