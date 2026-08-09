package auth_test

import (
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"testing"
)

func newCookieJar(t *testing.T) *cookiejar.Jar {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar.New: %v", err)
	}
	return jar
}

func mustParseURL(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	return u
}

func assertNoCookie(t *testing.T, resp *http.Response, name string) {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name == name && c.Value != "" {
			t.Fatalf("expected no %s cookie to be set, got value %q", name, c.Value)
		}
	}
}
