package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestStore backs the store with a CookieStore — same API as the Redis one,
// no server needed.
func newTestStore() *Store {
	return NewStoreWithBackend(sessions.NewCookieStore([]byte("test-secret-key")), SessionName)
}

// roundTrip replays the cookies a response set onto the next request.
func roundTrip(t *testing.T, rec *httptest.ResponseRecorder, req *http.Request) *http.Request {
	t.Helper()
	next := httptest.NewRequest(req.Method, req.URL.String(), nil)
	for _, c := range rec.Result().Cookies() {
		next.AddCookie(c)
	}
	return next
}

func TestSetAndValueRoundTrip(t *testing.T) {
	store := newTestStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	require.NoError(t, store.Set(rec, req, map[string]any{"clicks": int64(3)}))

	v, ok := store.Value(roundTrip(t, rec, req), "clicks")
	require.True(t, ok)
	assert.Equal(t, int64(3), v)
}

func TestValueMissingKey(t *testing.T) {
	_, ok := newTestStore().Value(httptest.NewRequest(http.MethodGet, "/", nil), "clicks")
	assert.False(t, ok)
}

func TestDeleteRemovesKey(t *testing.T) {
	store := newTestStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	require.NoError(t, store.Set(rec, req, map[string]any{"clicks": int64(3)}))

	req = roundTrip(t, rec, req)
	rec = httptest.NewRecorder()
	require.NoError(t, store.Delete(rec, req, "clicks"))

	_, ok := store.Value(roundTrip(t, rec, req), "clicks")
	assert.False(t, ok)
}

func TestClearExpiresTheCookie(t *testing.T) {
	store := newTestStore()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	require.NoError(t, store.Set(rec, req, map[string]any{"clicks": int64(3)}))

	req = roundTrip(t, rec, req)
	rec = httptest.NewRecorder()
	require.NoError(t, store.Clear(rec, req))

	cookies := rec.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Less(t, cookies[0].MaxAge, 0)
}
