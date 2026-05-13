package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIssueParseRoundtrip(t *testing.T) {
	tok, err := Issue("user-x", "admin", time.Minute)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	c, err := Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if c.Subject != "user-x" || c.Role != "admin" {
		t.Fatalf("unexpected claims: %+v", c)
	}
}

func TestParseExpired(t *testing.T) {
	tok, err := Issue("u", "admin", -time.Second)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := Parse(tok); err == nil {
		t.Fatalf("expected expired token to fail to parse")
	}
}

func TestParseGarbage(t *testing.T) {
	if _, err := Parse("not-a-jwt"); err == nil {
		t.Fatalf("expected error parsing garbage")
	}
}

func TestRequireAdminAllowsValidBearer(t *testing.T) {
	tok, err := Issue("admin", "admin", time.Minute)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	called := false
	h := RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("expected handler called and 200, called=%v code=%d", called, rec.Code)
	}
}

func TestRequireAdminMissingBearer(t *testing.T) {
	h := RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not be reached")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAdminRejectsNonAdminRole(t *testing.T) {
	tok, err := Issue("u", "player", time.Minute)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	h := RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not be reached")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestRequireAdminRejectsBadSignature(t *testing.T) {
	h := RequireAdmin(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler must not be reached")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	// Well-formed three-segment JWT shape, but bogus signature/payload.
	req.Header.Set("Authorization", "Bearer aaa.bbb.ccc")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
