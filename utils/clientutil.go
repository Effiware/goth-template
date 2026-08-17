package utils

import (
	"context"
	"net/http"
	"strings"
)

type mobileClientCtxKey struct{}

// IsMobileUA reports a phone-class browser via the "Mobi" UA token (MDN-recommended).
// Tablets deliberately read as desktop. A UX heuristic, never a security boundary.
func IsMobileUA(ua string) bool {
	return strings.Contains(ua, "Mobi")
}

// IsMobileClient prefers the Sec-CH-UA-Mobile client hint (Chromium), falling back
// to the User-Agent. "Request desktop site" clears both — the intended escape hatch.
func IsMobileClient(r *http.Request) bool {
	switch r.Header.Get("Sec-CH-UA-Mobile") {
	case "?1":
		return true
	case "?0":
		return false
	}
	return IsMobileUA(r.UserAgent())
}

// WithMobileClient stashes the per-request verdict; views read it via
// IsMobileClientFromContext (kept here so views need not import middlewares).
func WithMobileClient(ctx context.Context, mobile bool) context.Context {
	return context.WithValue(ctx, mobileClientCtxKey{}, mobile)
}

// IsMobileClientFromContext returns the stashed verdict; absent = desktop.
func IsMobileClientFromContext(ctx context.Context) bool {
	mobile, _ := ctx.Value(mobileClientCtxKey{}).(bool)
	return mobile
}
