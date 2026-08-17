package hda

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWithHTMLFallback(t *testing.T) {
	tests := map[string]struct {
		err        error
		wantStatus int
		wantBody   string
	}{
		"nil error leaves the response untouched": {nil, http.StatusOK, "rendered"},
		"unauthorized redirects":                  {&UnauthorizedError{Message: "no session"}, http.StatusUnauthorized, ""},
		"forbidden swaps a fragment":              {&ForbiddenError{Message: "nope"}, http.StatusForbidden, "do not have access"},
		"not found swaps a fragment":              {&NotFoundError{Message: "gone"}, http.StatusNotFound, "was not found"},
		"conflict shows its own message":          {&ConflictError{Message: "someone else edited this"}, http.StatusConflict, "someone else edited this"},
		"unknown error is a 500":                  {errors.New("boom"), http.StatusInternalServerError, "went wrong"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			handler := WithHTMLFallback(func(w http.ResponseWriter, _ *http.Request) error {
				if tc.err == nil {
					_, _ = w.Write([]byte("rendered"))
				}
				return tc.err
			})

			rec := httptest.NewRecorder()
			handler(rec, httptest.NewRequest(http.MethodPost, "/clicked", nil))

			assert.Equal(t, tc.wantStatus, rec.Code)
			assert.Contains(t, rec.Body.String(), tc.wantBody)
		})
	}
}

func TestWithHTMLFallbackRedirectsUnauthenticatedHTMX(t *testing.T) {
	handler := WithHTMLFallback(func(http.ResponseWriter, *http.Request) error {
		return &UnauthorizedError{Message: "no session"}
	})

	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, "/", rec.Header().Get("HX-Redirect"))
}

// An outerHTML swap deletes the target unless the fragment re-carries its id.
func TestWithHTMLFallbackKeepsHtmxTarget(t *testing.T) {
	handler := WithHTMLFallback(func(http.ResponseWriter, *http.Request) error {
		return errors.New("boom")
	})

	req := httptest.NewRequest(http.MethodPost, "/clicked", nil)
	req.Header.Set("HX-Target", "click-panel-counts")
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Contains(t, rec.Body.String(), `<div id="click-panel-counts">`)
}

// A boosted nav swaps #main-content by id — an error fragment missing that
// wrapper would delete the target instead of replacing it.
func TestWithHTMLFallbackWrapsBoostedFragment(t *testing.T) {
	handler := WithHTMLFallback(func(http.ResponseWriter, *http.Request) error {
		return &NotFoundError{Message: "gone"}
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("HX-Boosted", "true")
	rec := httptest.NewRecorder()
	handler(rec, req)

	assert.Contains(t, rec.Body.String(), `<div id="main-content">`)
}

func TestWithBrowserCache(t *testing.T) {
	handler := WithBrowserCache(time.Minute, func(http.ResponseWriter, *http.Request) error { return nil })

	rec := httptest.NewRecorder()
	_ = handler(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, "private, max-age=60", rec.Header().Get("Cache-Control"))
	assert.Equal(t, []string{"Cookie"}, rec.Header().Values("Vary"))
}
