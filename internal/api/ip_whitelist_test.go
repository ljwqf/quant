package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIPWhitelist_EmptyListAllowsAll(t *testing.T) {
	whitelist := NewIPWhitelist(nil, nil)
	if whitelist.enabled {
		t.Error("expected whitelist to be disabled with empty list")
	}

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	rec := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	whitelist.Middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestIPWhitelist_AllowedIP(t *testing.T) {
	rejected := false
	whitelist := NewIPWhitelist([]string{"192.168.1.0/24", "10.0.0.1"}, func(ip string) {
		rejected = true
	})

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "192.168.1.50:12345"
	rec := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	whitelist.Middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if rejected {
		t.Error("should not have rejected allowed IP")
	}
}

func TestIPWhitelist_BlockedIP(t *testing.T) {
	rejectedIP := ""
	whitelist := NewIPWhitelist([]string{"192.168.1.0/24"}, func(ip string) {
		rejectedIP = ip
	})

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "10.0.0.5:12345"
	rec := httptest.NewRecorder()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	whitelist.Middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
	if rejectedIP != "10.0.0.5" {
		t.Errorf("expected rejected IP '10.0.0.5', got '%s'", rejectedIP)
	}
}

func TestIPWhitelist_Update(t *testing.T) {
	whitelist := NewIPWhitelist([]string{"192.168.1.0/24"}, nil)

	// Block 10.0.0.1
	req := httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	whitelist.Middleware(handler).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 before update, got %d", rec.Code)
	}

	// Update to include 10.0.0.1
	whitelist.Update([]string{"10.0.0.0/8"})

	req = httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	rec = httptest.NewRecorder()
	whitelist.Middleware(handler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 after update, got %d", rec.Code)
	}
}

func TestIPWhitelist_InvalidCIDRSkipped(t *testing.T) {
	whitelist := NewIPWhitelist([]string{"invalid", "192.168.1.0/24"}, nil)

	// 192.168.1.0/24 should work
	req := httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "192.168.1.10:12345"
	rec := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	whitelist.Middleware(handler).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for valid CIDR, got %d", rec.Code)
	}
}

func TestIPWhitelist_XForwardedFor(t *testing.T) {
	whitelist := NewIPWhitelist([]string{"192.168.1.0/24"}, nil)

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "10.0.0.1:12345" // proxy IP
	req.Header.Set("X-Forwarded-For", "192.168.1.100, 10.0.0.1")
	rec := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	whitelist.Middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for X-Forwarded-For client IP, got %d", rec.Code)
	}
}

func TestIPWhitelist_XRealIP(t *testing.T) {
	whitelist := NewIPWhitelist([]string{"192.168.1.0/24"}, nil)

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.RemoteAddr = "10.0.0.1:12345"
	req.Header.Set("X-Real-IP", "192.168.1.200")
	rec := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	whitelist.Middleware(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for X-Real-IP client IP, got %d", rec.Code)
	}
}
