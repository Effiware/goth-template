package middlewares

import (
	"net/http"
	"net/url"
	"strings"
)

// SecurityHeadersOptions configures the SecurityHeaders middleware.
type SecurityHeadersOptions struct {
	// ExtraImgOrigins are img-src origins beyond 'self'/data: — e.g. an
	// object-storage presign origin. Empty entries are skipped.
	ExtraImgOrigins []string

	// ReportOnly emits the -Report-Only header instead (server.csp_report_only):
	// violations are logged, nothing is blocked. For soaking a policy change.
	ReportOnly bool
}

// SecurityHeaders sets the CSP and the standard hardening headers.
//
// script-src keeps 'unsafe-inline' (inline <script>/<style> in pageHead) and
// 'unsafe-eval' (Alpine evaluates x-data via the Function constructor);
// everything else is locked to 'self'. Assets are served locally, so there are
// no external origins — a view that adds one (a font CDN, an analytics script)
// must widen this policy in the same change.
func SecurityHeaders(opts SecurityHeadersOptions) func(http.Handler) http.Handler {
	// blob: covers client-side previews (URL.createObjectURL); only same-origin
	// script already running can mint one, so it is safer than data:.
	imgSrc := "img-src 'self' data: blob:"
	for _, origin := range opts.ExtraImgOrigins {
		if origin != "" {
			imgSrc += " " + origin
		}
	}

	policy := strings.Join([]string{
		"default-src 'self'",
		"script-src 'self' 'unsafe-inline' 'unsafe-eval'",
		"style-src 'self' 'unsafe-inline'",
		"font-src 'self'",
		imgSrc,
		"connect-src 'self'",
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'self'",
	}, "; ")

	cspHeader := "Content-Security-Policy"
	if opts.ReportOnly {
		cspHeader = "Content-Security-Policy-Report-Only"
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set(cspHeader, policy)
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			// Redundant with frame-ancestors when the CSP is enforcing, but
			// keeps clickjacking protection during a report-only soak.
			h.Set("X-Frame-Options", "SAMEORIGIN")
			next.ServeHTTP(w, r)
		})
	}
}

// OriginFromURL reduces a URL to scheme://host[:port] for a CSP source list;
// "" for unparseable or host-less input, which SecurityHeaders skips.
func OriginFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}
