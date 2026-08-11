package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

func TestSameOriginBehindReverseProxy(t *testing.T) {
	app := &App{cfg: Config{PublicBaseURL: "https://ledger.lay00.com/transfer"}}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1/login", nil)
	req.Host = "127.0.0.1:8081"
	req.Header.Set("Origin", "https://ledger.lay00.com:443")
	if !app.sameOrigin(req) {
		t.Fatal("configured public origin was rejected behind reverse proxy")
	}
	req.Header.Set("Origin", "https://evil.example")
	if app.sameOrigin(req) {
		t.Fatal("cross-origin request was accepted")
	}
	req.Header.Set("Origin", "null")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if !app.sameOrigin(req) {
		t.Fatal("same-origin browser request with opaque Origin was rejected")
	}
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	if app.sameOrigin(req) {
		t.Fatal("cross-site browser request with opaque Origin was accepted")
	}
}

func TestSecurityHeadersPreserveSameOriginFormOrigin(t *testing.T) {
	app := &App{cfg: Config{PublicBaseURL: "https://ledger.lay00.com/transfer"}}
	handler := app.securityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest(http.MethodGet, "https://ledger.lay00.com/login", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Fatalf("unexpected Referrer-Policy %q", got)
	}
	if got := rec.Header().Get("Content-Security-Policy"); !strings.Contains(got, "script-src 'self'") {
		t.Fatalf("admin script is not allowed by CSP: %q", got)
	}
}

func TestAdminPageIncludesUploadProgressUI(t *testing.T) {
	recorder := httptest.NewRecorder()
	render(recorder, adminTemplate, adminView{BasePath: "/transfer", CSRF: "csrf", MaxUpload: "5.0 GiB"})
	html := recorder.Body.String()
	for _, expected := range []string{"id=\"upload-form\"", "id=\"upload-progress\"", "id=\"upload-bar\"", "/transfer/assets/admin-v2.js"} {
		if !strings.Contains(html, expected) {
			t.Fatalf("admin page missing %q", expected)
		}
	}
	if !strings.Contains(adminJS, "request.upload.addEventListener") || !strings.Contains(adminJS, "/chunk/") || !strings.Contains(adminJS, "retryChunk") {
		t.Fatal("upload progress handler missing")
	}
}

func TestChunkedUploadCreatesShare(t *testing.T) {
	dir := t.TempDir()
	store, err := openStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	key := []byte("01234567890123456789012345678901")
	app := &App{cfg: Config{
		DataDir: dir, BasePath: "/transfer", PublicBaseURL: "https://example.test/transfer",
		SigningKey: key, MaxUploadBytes: 1024 * 1024, DefaultExpiry: time.Hour, MaxExpiry: 24 * time.Hour,
	}, store: store, loginLimiter: newLoginLimiter()}
	handler := app.routes()
	csrf := "test-csrf"
	token, err := signToken(key, signedToken{Kind: "admin", CSRF: csrf, Expires: time.Now().Add(time.Hour).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	adminCookie := &http.Cookie{Name: "relay_admin", Value: token, Path: "/transfer/"}
	payload := []byte("chunked upload content")

	initForm := url.Values{"csrf": {csrf}, "name": {"report.txt"}, "size": {strconv.Itoa(len(payload))}, "expires_hours": {"2"}}
	initReq := httptest.NewRequest(http.MethodPost, "https://example.test/admin/upload/init", strings.NewReader(initForm.Encode()))
	initReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	initReq.Header.Set("Origin", "https://example.test")
	initReq.AddCookie(adminCookie)
	initRec := httptest.NewRecorder()
	handler.ServeHTTP(initRec, initReq)
	if initRec.Code != http.StatusCreated {
		t.Fatalf("init status %d: %s", initRec.Code, initRec.Body.String())
	}
	var initialized struct {
		ID     string `json:"id"`
		Offset int64  `json:"offset"`
	}
	if err := json.Unmarshal(initRec.Body.Bytes(), &initialized); err != nil || initialized.ID == "" {
		t.Fatalf("invalid init response: %v, %s", err, initRec.Body.String())
	}

	chunkReq := httptest.NewRequest(http.MethodPost, "https://example.test/admin/upload/chunk/"+initialized.ID, bytes.NewReader(payload))
	chunkReq.Header.Set("Origin", "https://example.test")
	chunkReq.Header.Set("X-CSRF-Token", csrf)
	chunkReq.Header.Set("X-Upload-Offset", "0")
	chunkReq.AddCookie(adminCookie)
	chunkRec := httptest.NewRecorder()
	handler.ServeHTTP(chunkRec, chunkReq)
	if chunkRec.Code != http.StatusOK {
		t.Fatalf("chunk status %d: %s", chunkRec.Code, chunkRec.Body.String())
	}
	replayReq := httptest.NewRequest(http.MethodPost, "https://example.test/admin/upload/chunk/"+initialized.ID, bytes.NewReader(payload))
	replayReq.Header.Set("Origin", "https://example.test")
	replayReq.Header.Set("X-CSRF-Token", csrf)
	replayReq.Header.Set("X-Upload-Offset", "0")
	replayReq.AddCookie(adminCookie)
	replayRec := httptest.NewRecorder()
	handler.ServeHTTP(replayRec, replayReq)
	if replayRec.Code != http.StatusConflict || !strings.Contains(replayRec.Body.String(), strconv.Itoa(len(payload))) {
		t.Fatalf("replayed chunk did not report current offset: %d %s", replayRec.Code, replayRec.Body.String())
	}

	finishForm := url.Values{"csrf": {csrf}}
	finishReq := httptest.NewRequest(http.MethodPost, "https://example.test/admin/upload/finish/"+initialized.ID, strings.NewReader(finishForm.Encode()))
	finishReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	finishReq.Header.Set("Origin", "https://example.test")
	finishReq.AddCookie(adminCookie)
	finishRec := httptest.NewRecorder()
	handler.ServeHTTP(finishRec, finishReq)
	if finishRec.Code != http.StatusCreated {
		t.Fatalf("finish status %d: %s", finishRec.Code, finishRec.Body.String())
	}
	shares := store.list()
	if len(shares) != 1 || shares[0].OriginalName != "report.txt" || shares[0].Size != int64(len(payload)) {
		t.Fatalf("unexpected shares: %+v", shares)
	}
	stored, err := os.ReadFile(filepath.Join(dir, shares[0].StoredName))
	if err != nil || !bytes.Equal(stored, payload) {
		t.Fatalf("stored file mismatch: %v", err)
	}
}
