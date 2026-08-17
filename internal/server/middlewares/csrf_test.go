package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSRFOriginCheck(t *testing.T) {
	tests := map[string]struct {
		method     string
		origin     string
		referer    string
		wantStatus int
	}{
		"GET passes without origin":     {http.MethodGet, "", "", http.StatusOK},
		"exact origin passes":           {http.MethodPost, "https://app.example.com", "", http.StatusOK},
		"referer under origin passes":   {http.MethodPost, "", "https://app.example.com/page", http.StatusOK},
		"missing origin rejected":       {http.MethodPost, "", "", http.StatusForbidden},
		"foreign origin rejected":       {http.MethodPost, "https://evil.net", "", http.StatusForbidden},
		"prefix-suffix origin rejected": {http.MethodPost, "https://app.example.com.evil.net", "", http.StatusForbidden},
	}

	handler := CSRFOriginCheck("https://app.example.com")(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/clicked", nil)
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantStatus, rec.Code)
		})
	}
}

// A swapped 403 must re-carry the htmx target id, or the swap deletes the target.
func TestCSRFOriginCheckKeepsHtmxTarget(t *testing.T) {
	handler := CSRFOriginCheck("https://app.example.com")(
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "/clicked", nil)
	req.Header.Set("Origin", "https://evil.net")
	req.Header.Set("HX-Target", "click-panel-counts")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, rec.Body.String(), `<div id="click-panel-counts">`)
}
