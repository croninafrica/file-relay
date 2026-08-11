package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPasswordHash(t *testing.T) {
	hash, err := hashPassword("correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	if !verifyPassword(hash, "correct horse battery staple") {
		t.Fatal("valid password rejected")
	}
	if verifyPassword(hash, "wrong password") {
		t.Fatal("invalid password accepted")
	}
}

func TestSignedToken(t *testing.T) {
	key := []byte("01234567890123456789012345678901")
	now := time.Now()
	value, err := signToken(key, signedToken{Kind: "download", ShareID: "abc", Expires: now.Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := verifyToken(key, value, "download", now)
	if !ok || got.ShareID != "abc" {
		t.Fatal("valid token rejected")
	}
	if _, ok := verifyToken(key, value+"x", "download", now); ok {
		t.Fatal("tampered token accepted")
	}
	if _, ok := verifyToken(key, value, "download", now.Add(2*time.Minute)); ok {
		t.Fatal("expired token accepted")
	}
}

func TestStorePasswordLockoutAndDownloadLimit(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	hash, err := hashPassword("a secure share password")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	share := &Share{ID: "share", OriginalName: "a.txt", StoredName: "share.blob", CreatedAt: now, ExpiresAt: now.Add(time.Hour), PasswordHash: hash}
	if err := store.create(share); err != nil {
		t.Fatal(err)
	}
	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := store.authorize("share", "ip-a", "wrong password", now, 3, 15*time.Minute, 30*time.Minute, 3); err != errBadPassword {
			t.Fatalf("attempt %d: got %v", attempt, err)
		}
	}
	if _, err := store.authorize("share", "ip-a", "wrong password", now, 3, 15*time.Minute, 30*time.Minute, 3); err != errLocked {
		t.Fatalf("third attempt should lock, got %v", err)
	}
	for download := 1; download <= 3; download++ {
		if _, err := store.authorize("share", "ip-b", "a secure share password", now, 3, 15*time.Minute, 30*time.Minute, 3); err != nil {
			t.Fatalf("download %d: %v", download, err)
		}
	}
	if _, err := store.authorize("share", "ip-b", "a secure share password", now, 3, 15*time.Minute, 30*time.Minute, 3); err != errDownloadLimit {
		t.Fatalf("fourth download should be blocked, got %v", err)
	}
	reloaded, err := openStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := reloaded.get("share")
	if err != nil || got.Downloads["ip-b"] != 3 {
		t.Fatalf("state was not persisted: %+v, %v", got.Downloads, err)
	}
}

func TestAuthorizeThenInternalDownload(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := store.create(&Share{ID: "public-share", OriginalName: "report 2026.txt", StoredName: "public-share.blob", Size: 42, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	app := &App{cfg: Config{
		BasePath: "/transfer", PublicBaseURL: "https://example.test/transfer", SigningKey: []byte("01234567890123456789012345678901"),
		IPHashKey: []byte("abcdefghijklmnopqrstuvwxyz123456"), MaxAttempts: 3, AttemptWindow: 15 * time.Minute, Lockout: 30 * time.Minute,
		MaxDownloadsByIP: 3, SecureCookies: true,
	}, store: store, loginLimiter: newLoginLimiter()}
	handler := app.routes()

	form := url.Values{}
	req := httptest.NewRequest(http.MethodPost, "https://example.test/s/public-share/authorize", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://example.test")
	req.Header.Set("X-Real-IP", "203.0.113.8")
	req.RemoteAddr = "127.0.0.1:10000"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("authorize status %d: %s", rec.Code, rec.Body.String())
	}
	result := rec.Result()
	var downloadCookie *http.Cookie
	for _, cookie := range result.Cookies() {
		if strings.HasPrefix(cookie.Name, "relay_dl_") {
			downloadCookie = cookie
		}
	}
	if downloadCookie == nil {
		t.Fatal("download cookie missing")
	}

	downloadReq := httptest.NewRequest(http.MethodGet, "https://example.test/d/public-share", nil)
	downloadReq.Header.Set("X-Real-IP", "203.0.113.8")
	downloadReq.RemoteAddr = "127.0.0.1:10001"
	downloadReq.AddCookie(downloadCookie)
	downloadRec := httptest.NewRecorder()
	handler.ServeHTTP(downloadRec, downloadReq)
	if downloadRec.Code != http.StatusOK {
		t.Fatalf("download status %d: %s", downloadRec.Code, downloadRec.Body.String())
	}
	if got := downloadRec.Header().Get("X-Accel-Redirect"); got != "/_file_relay_internal/public-share.blob" {
		t.Fatalf("unexpected internal redirect %q", got)
	}
	if got := downloadRec.Header().Get("Content-Disposition"); !strings.Contains(got, "report 2026.txt") {
		t.Fatalf("unexpected content disposition %q", got)
	}
}

func TestSafeFilename(t *testing.T) {
	if got := safeFilename("../../secret.txt"); got != "secret.txt" {
		t.Fatalf("path traversal was not removed: %q", got)
	}
	if got := safeFilename("bad\r\nname.txt"); got != "badname.txt" {
		t.Fatalf("control characters were not removed: %q", got)
	}
}
