package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJsonHandler(t *testing.T) {
	tests := map[string]struct {
		handler      EndpointHandlerT
		wantStatus   int
		wantBody     string
		wantNoneType bool // no Content-Type on bodiless responses
	}{
		"payload is marshaled": {
			handler: func(http.ResponseWriter, *http.Request) (int, any, error) {
				return http.StatusOK, Clicks{Count: 7}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `{"count":7}`,
		},
		"error becomes a bodiless 500": {
			handler:      func(http.ResponseWriter, *http.Request) (int, any, error) { return 0, nil, errors.New("boom") },
			wantStatus:   http.StatusInternalServerError,
			wantNoneType: true,
		},
		"204 carries no body": {
			handler:      func(http.ResponseWriter, *http.Request) (int, any, error) { return http.StatusNoContent, Clicks{}, nil },
			wantStatus:   http.StatusNoContent,
			wantNoneType: true,
		},
		"nil payload carries no body": {
			handler:      func(http.ResponseWriter, *http.Request) (int, any, error) { return http.StatusAccepted, nil, nil },
			wantStatus:   http.StatusAccepted,
			wantNoneType: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			JsonHandler(tc.handler)(rec, httptest.NewRequest(http.MethodGet, "/", nil))

			assert.Equal(t, tc.wantStatus, rec.Code)
			if tc.wantNoneType {
				assert.Empty(t, rec.Body.String())
				assert.Empty(t, rec.Header().Get("Content-Type"))
				return
			}
			assert.JSONEq(t, tc.wantBody, rec.Body.String())
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		})
	}
}

func TestReadyzReportsDraining(t *testing.T) {
	t.Cleanup(func() { draining.Store(false) })
	BeginDraining()

	// Draining short-circuits before any dependency is touched, so nil deps are fine.
	code, payload, err := Readyz(nil, nil)(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/readyz", nil))

	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, code)
	assert.Equal(t, map[string]string{"status": "draining"}, payload)
}

func TestIsProbePath(t *testing.T) {
	for path, want := range map[string]bool{"/readyz": true, "/ping": true, "/": false, "/api/v1/clicks": false} {
		assert.Equal(t, want, IsProbePath(httptest.NewRequest(http.MethodGet, path, nil)), path)
	}
}
