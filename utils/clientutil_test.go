package utils

import (
	"net/http/httptest"
	"testing"
)

func TestIsMobileUA(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want bool
	}{
		{"iPhone Safari", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_5 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Mobile/15E148 Safari/604.1", true},
		{"Android Chrome", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Mobile Safari/537.36", true},
		{"Firefox Android", "Mozilla/5.0 (Android 14; Mobile; rv:127.0) Gecko/127.0 Firefox/127.0", true},
		{"iPad desktop-class UA", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.5 Safari/605.1.15", false},
		{"Windows Chrome", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsMobileUA(tc.ua); got != tc.want {
				t.Errorf("IsMobileUA(%q) = %v, want %v", tc.ua, got, tc.want)
			}
		})
	}
}

func TestIsMobileClient(t *testing.T) {
	cases := []struct {
		name string
		hint string
		ua   string
		want bool
	}{
		{"hint mobile wins over desktop UA", "?1", "Mozilla/5.0 (Windows NT 10.0)", true},
		{"hint desktop wins over mobile UA", "?0", "Mozilla/5.0 (Android 14; Mobile)", false},
		{"no hint falls back to mobile UA", "", "Mozilla/5.0 (Android 14; Mobile)", true},
		{"no hint falls back to desktop UA", "", "Mozilla/5.0 (Windows NT 10.0)", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.Header.Set("User-Agent", tc.ua)
			if tc.hint != "" {
				r.Header.Set("Sec-CH-UA-Mobile", tc.hint)
			}
			if got := IsMobileClient(r); got != tc.want {
				t.Errorf("IsMobileClient = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMobileClientContext(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	if IsMobileClientFromContext(r.Context()) {
		t.Error("absent flag should read as desktop")
	}
	ctx := WithMobileClient(r.Context(), true)
	if !IsMobileClientFromContext(ctx) {
		t.Error("stashed true should read back true")
	}
}
