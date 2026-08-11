package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Config struct {
	ListenAddr       string
	DataDir          string
	StateFile        string
	BasePath         string
	PublicBaseURL    string
	AdminHash        string
	SigningKey       []byte
	IPHashKey        []byte
	MaxUploadBytes   int64
	DefaultExpiry    time.Duration
	MaxExpiry        time.Duration
	MaxAttempts      int
	AttemptWindow    time.Duration
	Lockout          time.Duration
	MaxDownloadsByIP int
	SecureCookies    bool
}

func loadConfig() (Config, error) {
	cfg := Config{
		ListenAddr:       envOr("LISTEN_ADDR", "127.0.0.1:8081"),
		DataDir:          envOr("DATA_DIR", "/var/lib/file-relay/files"),
		StateFile:        envOr("STATE_FILE", "/var/lib/file-relay/state.json"),
		BasePath:         strings.TrimRight(envOr("BASE_PATH", "/transfer"), "/"),
		PublicBaseURL:    strings.TrimRight(os.Getenv("PUBLIC_BASE_URL"), "/"),
		AdminHash:        os.Getenv("ADMIN_PASSWORD_HASH"),
		MaxUploadBytes:   int64Env("MAX_UPLOAD_BYTES", 5*1024*1024*1024),
		DefaultExpiry:    time.Duration(parsePositiveInt(os.Getenv("DEFAULT_EXPIRY_HOURS"), 24)) * time.Hour,
		MaxExpiry:        time.Duration(parsePositiveInt(os.Getenv("MAX_EXPIRY_HOURS"), 720)) * time.Hour,
		MaxAttempts:      parsePositiveInt(os.Getenv("MAX_PASSWORD_ATTEMPTS"), 3),
		AttemptWindow:    time.Duration(parsePositiveInt(os.Getenv("ATTEMPT_WINDOW_MINUTES"), 15)) * time.Minute,
		Lockout:          time.Duration(parsePositiveInt(os.Getenv("LOCKOUT_MINUTES"), 30)) * time.Minute,
		MaxDownloadsByIP: parsePositiveInt(os.Getenv("MAX_DOWNLOADS_PER_IP"), 3),
		SecureCookies:    envOr("SECURE_COOKIES", "true") != "false",
	}
	if cfg.BasePath == "" || !strings.HasPrefix(cfg.BasePath, "/") {
		return Config{}, errors.New("BASE_PATH must start with /")
	}
	if cfg.PublicBaseURL == "" {
		return Config{}, errors.New("PUBLIC_BASE_URL is required")
	}
	if cfg.AdminHash == "" {
		return Config{}, errors.New("ADMIN_PASSWORD_HASH is required")
	}
	var err error
	if cfg.SigningKey, err = decodeKey("SIGNING_KEY"); err != nil {
		return Config{}, err
	}
	if cfg.IPHashKey, err = decodeKey("IP_HASH_KEY"); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func decodeKey(name string) ([]byte, error) {
	value := os.Getenv(name)
	key, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(key) < 32 {
		return nil, fmt.Errorf("%s must be at least 32 random bytes in unpadded base64", name)
	}
	return key, nil
}

func int64Env(name string, fallback int64) int64 {
	n, err := strconv.ParseInt(os.Getenv(name), 10, 64)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

type App struct {
	cfg          Config
	store        *Store
	loginMu      sync.Mutex
	loginLimiter *loginLimiter
	uploadMu     sync.Mutex
}

const uploadChunkSize int64 = 8 * 1024 * 1024

type pendingUpload struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	Size         int64     `json:"size"`
	PasswordHash string    `json:"password_hash,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "hash-password" {
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() {
			log.Fatal("read password from stdin")
		}
		hash, err := hashPassword(strings.TrimSpace(scanner.Text()))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(hash)
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		log.Fatal(err)
	}
	store, err := openStore(cfg.StateFile)
	if err != nil {
		log.Fatal(err)
	}
	app := &App{cfg: cfg, store: store, loginLimiter: newLoginLimiter()}
	app.cleanupExpired()
	go app.cleanupLoop()

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           app.routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		log.Printf("file-relay listening on %s", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /assets/admin.js", a.handleAdminJS)
	mux.HandleFunc("GET /assets/admin-v2.js", a.handleAdminJS)
	mux.HandleFunc("GET /", a.handleRoot)
	mux.HandleFunc("GET /admin/login", a.handleLoginPage)
	mux.HandleFunc("POST /admin/login", a.handleLogin)
	mux.HandleFunc("POST /admin/logout", a.handleLogout)
	mux.HandleFunc("GET /admin", a.handleAdmin)
	mux.HandleFunc("POST /admin/upload", a.handleUpload)
	mux.HandleFunc("POST /admin/upload/init", a.handleUploadInit)
	mux.HandleFunc("POST /admin/upload/chunk/{id}", a.handleUploadChunk)
	mux.HandleFunc("POST /admin/upload/finish/{id}", a.handleUploadFinish)
	mux.HandleFunc("POST /admin/delete", a.handleDelete)
	mux.HandleFunc("GET /s/{id}", a.handleShare)
	mux.HandleFunc("POST /s/{id}/authorize", a.handleAuthorize)
	mux.HandleFunc("GET /d/{id}", a.handleDownload)
	mux.HandleFunc("HEAD /d/{id}", a.handleDownload)
	return a.securityHeaders(mux)
}

func (a *App) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'self'; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		if r.Method == http.MethodPost && !a.sameOrigin(r) {
			log.Printf("blocked cross-origin request: origin=%q host=%q forwarded_proto=%q fetch_site=%q fetch_mode=%q path=%q", r.Header.Get("Origin"), r.Host, r.Header.Get("X-Forwarded-Proto"), r.Header.Get("Sec-Fetch-Site"), r.Header.Get("Sec-Fetch-Mode"), r.URL.Path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if origin == "null" {
		return r.Header.Get("Sec-Fetch-Site") == "same-origin"
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return false
	}
	publicURL, publicErr := url.Parse(a.cfg.PublicBaseURL)
	if publicErr == nil && strings.EqualFold(u.Scheme, publicURL.Scheme) && strings.EqualFold(u.Hostname(), publicURL.Hostname()) {
		return true
	}
	requestHost := r.Host
	if host, _, splitErr := net.SplitHostPort(r.Host); splitErr == nil {
		requestHost = host
	}
	return strings.EqualFold(u.Hostname(), strings.Trim(requestHost, "[]"))
}

func (a *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok\n")
}

func (a *App) handleAdminJS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = io.WriteString(w, adminJS)
}

func (a *App) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	http.Redirect(w, r, a.cfg.BasePath+"/admin", http.StatusFound)
}

func (a *App) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.adminSession(r); ok {
		http.Redirect(w, r, a.cfg.BasePath+"/admin", http.StatusFound)
		return
	}
	render(w, loginTemplate, loginView{BasePath: a.cfg.BasePath})
}

func (a *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ip := clientIP(r)
	now := time.Now()
	a.loginMu.Lock()
	allowed := a.loginLimiter.allow(ip, now)
	a.loginMu.Unlock()
	if !allowed {
		renderStatus(w, loginTemplate, loginView{BasePath: a.cfg.BasePath, Error: "尝试次数过多，请稍后再试。"}, http.StatusTooManyRequests)
		return
	}
	if !verifyPassword(a.cfg.AdminHash, r.FormValue("password")) {
		a.loginMu.Lock()
		a.loginLimiter.fail(ip, now)
		a.loginMu.Unlock()
		time.Sleep(400 * time.Millisecond)
		renderStatus(w, loginTemplate, loginView{BasePath: a.cfg.BasePath, Error: "登录失败。"}, http.StatusUnauthorized)
		return
	}
	a.loginMu.Lock()
	a.loginLimiter.clear(ip)
	a.loginMu.Unlock()
	csrf, err := randomToken(24)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	token, err := signToken(a.cfg.SigningKey, signedToken{Kind: "admin", CSRF: csrf, Expires: now.Add(12 * time.Hour).Unix()})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "relay_admin", Value: token, Path: a.cfg.BasePath + "/", MaxAge: 12 * 3600, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, a.cfg.BasePath+"/admin", http.StatusSeeOther)
}

func (a *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	session, ok := a.adminSession(r)
	if !ok || r.FormValue("csrf") != session.CSRF {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "relay_admin", Value: "", Path: a.cfg.BasePath + "/", MaxAge: -1, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, a.cfg.BasePath+"/admin/login", http.StatusSeeOther)
}

func (a *App) adminSession(r *http.Request) (signedToken, bool) {
	cookie, err := r.Cookie("relay_admin")
	if err != nil {
		return signedToken{}, false
	}
	return verifyToken(a.cfg.SigningKey, cookie.Value, "admin", time.Now())
}

func (a *App) requireAdmin(w http.ResponseWriter, r *http.Request) (signedToken, bool) {
	session, ok := a.adminSession(r)
	if !ok {
		http.Redirect(w, r, a.cfg.BasePath+"/admin/login", http.StatusFound)
		return signedToken{}, false
	}
	return session, true
}

func (a *App) handleAdmin(w http.ResponseWriter, r *http.Request) {
	session, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	now := time.Now()
	shares := a.store.list()
	items := make([]adminShareView, 0, len(shares))
	for _, share := range shares {
		downloads := 0
		for _, count := range share.Downloads {
			downloads += count
		}
		items = append(items, adminShareView{
			ID: share.ID, Name: share.OriginalName, Size: humanBytes(share.Size),
			Created: share.CreatedAt.Local().Format("2006-01-02 15:04"), Expires: share.ExpiresAt.Local().Format("2006-01-02 15:04"),
			URL: a.cfg.PublicBaseURL + "/s/" + share.ID, Protected: share.PasswordHash != "", Downloads: downloads, Expired: !share.ExpiresAt.After(now),
		})
	}
	notice := ""
	if r.URL.Query().Get("uploaded") == "1" {
		notice = "上传完成，分享链接已创建。"
	} else if r.URL.Query().Get("deleted") == "1" {
		notice = "分享及文件已删除。"
	}
	render(w, adminTemplate, adminView{BasePath: a.cfg.BasePath, CSRF: session.CSRF, Shares: items, MaxUpload: humanBytes(a.cfg.MaxUploadBytes), Notice: notice})
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	session, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.cfg.MaxUploadBytes+1024*1024)
	mr, err := r.MultipartReader()
	if err != nil {
		http.Error(w, "invalid upload", http.StatusBadRequest)
		return
	}
	tmp, err := os.CreateTemp(a.cfg.DataDir, ".upload-*")
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	hasher := sha256.New()
	var originalName, password, csrf, expiryValue string
	var size int64
	fileSeen := false
	for {
		part, nextErr := mr.NextPart()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			_ = tmp.Close()
			http.Error(w, "upload interrupted", http.StatusBadRequest)
			return
		}
		if part.FileName() != "" {
			if fileSeen {
				_ = part.Close()
				_ = tmp.Close()
				http.Error(w, "only one file is allowed", http.StatusBadRequest)
				return
			}
			fileSeen = true
			originalName = safeFilename(part.FileName())
			size, err = io.Copy(io.MultiWriter(tmp, hasher), part)
			_ = part.Close()
			if err != nil {
				_ = tmp.Close()
				http.Error(w, "upload failed", http.StatusBadRequest)
				return
			}
			continue
		}
		value, readErr := io.ReadAll(io.LimitReader(part, 4097))
		_ = part.Close()
		if readErr != nil || len(value) > 4096 {
			_ = tmp.Close()
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		switch part.FormName() {
		case "password":
			password = string(value)
		case "csrf":
			csrf = string(value)
		case "expires_hours":
			expiryValue = string(value)
		}
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if err := tmp.Close(); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if csrf != session.CSRF || !fileSeen || originalName == "" {
		http.Error(w, "invalid upload", http.StatusBadRequest)
		return
	}
	if size > a.cfg.MaxUploadBytes {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	expires := a.cfg.DefaultExpiry
	if hours, parseErr := strconv.Atoi(strings.TrimSpace(expiryValue)); parseErr == nil && hours > 0 {
		expires = time.Duration(hours) * time.Hour
	}
	if expires > a.cfg.MaxExpiry {
		expires = a.cfg.MaxExpiry
	}
	var passwordHash string
	if password != "" {
		passwordHash, err = hashPassword(password)
		if err != nil {
			http.Error(w, "password must be at least 12 characters", http.StatusBadRequest)
			return
		}
	}
	id, err := randomToken(24)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	storedName := id + ".blob"
	finalPath := filepath.Join(a.cfg.DataDir, storedName)
	if err := os.Chmod(tmpName, 0640); err != nil || os.Rename(tmpName, finalPath) != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC()
	share := &Share{ID: id, OriginalName: originalName, StoredName: storedName, Size: size, SHA256: hex.EncodeToString(hasher.Sum(nil)), CreatedAt: now, ExpiresAt: now.Add(expires), PasswordHash: passwordHash}
	if err := a.store.create(share); err != nil {
		_ = os.Remove(finalPath)
		http.Error(w, "metadata error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, a.cfg.BasePath+"/admin?uploaded=1", http.StatusSeeOther)
}

func (a *App) handleUploadInit(w http.ResponseWriter, r *http.Request) {
	session, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64*1024)
	if err := r.ParseForm(); err != nil || r.FormValue("csrf") != session.CSRF {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	name := safeFilename(r.FormValue("name"))
	size, err := strconv.ParseInt(r.FormValue("size"), 10, 64)
	if err != nil || size <= 0 || size > a.cfg.MaxUploadBytes || name == "" {
		http.Error(w, "invalid upload", http.StatusBadRequest)
		return
	}
	passwordHash := ""
	if password := r.FormValue("password"); password != "" {
		passwordHash, err = hashPassword(password)
		if err != nil {
			http.Error(w, "password must be at least 12 characters", http.StatusBadRequest)
			return
		}
	}
	expires := a.cfg.DefaultExpiry
	if hours, parseErr := strconv.Atoi(strings.TrimSpace(r.FormValue("expires_hours"))); parseErr == nil && hours > 0 {
		expires = time.Duration(hours) * time.Hour
	}
	if expires > a.cfg.MaxExpiry {
		expires = a.cfg.MaxExpiry
	}
	id, err := randomToken(24)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	pending := pendingUpload{ID: id, OriginalName: name, Size: size, PasswordHash: passwordHash, CreatedAt: time.Now().UTC()}
	pending.ExpiresAt = pending.CreatedAt.Add(expires)
	partPath, metaPath, ok := a.pendingUploadPaths(id)
	if !ok {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	file, err := os.OpenFile(partPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if closeErr := file.Close(); closeErr != nil {
		_ = os.Remove(partPath)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	raw, err := json.Marshal(pending)
	if err != nil || os.WriteFile(metaPath, raw, 0600) != nil {
		_ = os.Remove(partPath)
		_ = os.Remove(metaPath)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "offset": int64(0), "chunk_size": uploadChunkSize})
}

func (a *App) handleUploadChunk(w http.ResponseWriter, r *http.Request) {
	session, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	if r.Header.Get("X-CSRF-Token") != session.CSRF {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	offset, err := strconv.ParseInt(r.Header.Get("X-Upload-Offset"), 10, 64)
	if err != nil || offset < 0 {
		http.Error(w, "invalid offset", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	pending, partPath, _, err := a.loadPendingUpload(id)
	if err != nil {
		http.Error(w, "upload not found", http.StatusNotFound)
		return
	}
	info, err := os.Stat(partPath)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	actual := info.Size()
	if offset != actual {
		writeJSON(w, http.StatusConflict, map[string]any{"offset": actual})
		return
	}
	remaining := pending.Size - actual
	if remaining <= 0 {
		writeJSON(w, http.StatusOK, map[string]any{"offset": actual})
		return
	}
	limit := uploadChunkSize
	if remaining < limit {
		limit = remaining
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit+1)
	file, err := os.OpenFile(partPath, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	written, copyErr := io.Copy(file, io.LimitReader(r.Body, limit+1))
	if copyErr != nil || written <= 0 || written > limit {
		_ = file.Close()
		_ = os.Truncate(partPath, actual)
		http.Error(w, "invalid chunk", http.StatusBadRequest)
		return
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		_ = os.Truncate(partPath, actual)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if err := file.Close(); err != nil {
		_ = os.Truncate(partPath, actual)
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"offset": actual + written})
}

func (a *App) handleUploadFinish(w http.ResponseWriter, r *http.Request) {
	session, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	if err := r.ParseForm(); err != nil || r.FormValue("csrf") != session.CSRF {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	pending, partPath, metaPath, err := a.loadPendingUpload(id)
	if err != nil {
		http.Error(w, "upload not found", http.StatusNotFound)
		return
	}
	file, err := os.Open(partPath)
	if err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	info, err := file.Stat()
	if err != nil || info.Size() != pending.Size {
		_ = file.Close()
		http.Error(w, "upload incomplete", http.StatusConflict)
		return
	}
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		_ = file.Close()
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	if err := file.Close(); err != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	shareID, err := randomToken(24)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	storedName := shareID + ".blob"
	finalPath := filepath.Join(a.cfg.DataDir, storedName)
	if err := os.Chmod(partPath, 0640); err != nil || os.Rename(partPath, finalPath) != nil {
		http.Error(w, "storage error", http.StatusInternalServerError)
		return
	}
	share := &Share{ID: shareID, OriginalName: pending.OriginalName, StoredName: storedName, Size: pending.Size, SHA256: hex.EncodeToString(hasher.Sum(nil)), CreatedAt: pending.CreatedAt, ExpiresAt: pending.ExpiresAt, PasswordHash: pending.PasswordHash}
	if err := a.store.create(share); err != nil {
		_ = os.Rename(finalPath, partPath)
		http.Error(w, "metadata error", http.StatusInternalServerError)
		return
	}
	if err := os.Remove(metaPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("remove completed upload metadata %s: %v", id, err)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"success_url": a.cfg.BasePath + "/admin?uploaded=1"})
}

func (a *App) pendingUploadPaths(id string) (string, string, bool) {
	if len(id) != 32 {
		return "", "", false
	}
	for _, char := range id {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '-' && char != '_' {
			return "", "", false
		}
	}
	base := filepath.Join(a.cfg.DataDir, ".upload-"+id)
	return base + ".part", base + ".json", true
}

func (a *App) loadPendingUpload(id string) (pendingUpload, string, string, error) {
	partPath, metaPath, ok := a.pendingUploadPaths(id)
	if !ok {
		return pendingUpload{}, "", "", errors.New("invalid upload id")
	}
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return pendingUpload{}, "", "", err
	}
	var pending pendingUpload
	if err := json.Unmarshal(raw, &pending); err != nil || pending.ID != id || pending.Size <= 0 || pending.Size > a.cfg.MaxUploadBytes {
		return pendingUpload{}, "", "", errors.New("invalid upload metadata")
	}
	return pending, partPath, metaPath, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	session, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil || r.FormValue("csrf") != session.CSRF {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	share, err := a.store.delete(r.FormValue("id"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	_ = os.Remove(filepath.Join(a.cfg.DataDir, share.StoredName))
	http.Redirect(w, r, a.cfg.BasePath+"/admin?deleted=1", http.StatusSeeOther)
}

func (a *App) handleShare(w http.ResponseWriter, r *http.Request) {
	share, err := a.store.get(r.PathValue("id"))
	if err != nil || !share.ExpiresAt.After(time.Now()) {
		renderStatus(w, messageTemplate, messageView{Title: "链接不可用", Message: "该分享不存在或已过期。"}, http.StatusNotFound)
		return
	}
	render(w, shareTemplate, shareView{BasePath: a.cfg.BasePath, ID: share.ID, Name: share.OriginalName, Size: humanBytes(share.Size), Expires: share.ExpiresAt.Local().Format("2006-01-02 15:04"), Protected: share.PasswordHash != ""})
}

func (a *App) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id := r.PathValue("id")
	key := ipKey(a.cfg.IPHashKey, id, clientIP(r))
	share, err := a.store.authorize(id, key, r.FormValue("password"), time.Now(), a.cfg.MaxAttempts, a.cfg.AttemptWindow, a.cfg.Lockout, a.cfg.MaxDownloadsByIP)
	if err != nil {
		status := http.StatusUnauthorized
		message := "密码错误或链接不可用。"
		if errors.Is(err, errLocked) {
			status, message = http.StatusTooManyRequests, "尝试次数过多，请稍后再试。"
		} else if errors.Is(err, errDownloadLimit) {
			status, message = http.StatusTooManyRequests, "此网络地址的下载次数已用完。"
		} else if errors.Is(err, errShareExpired) || errors.Is(err, errShareNotFound) {
			status, message = http.StatusNotFound, "该分享不存在或已过期。"
		}
		renderStatus(w, messageTemplate, messageView{Title: "无法下载", Message: message}, status)
		return
	}
	token, err := signToken(a.cfg.SigningKey, signedToken{Kind: "download", ShareID: id, IPKey: key, Expires: time.Now().Add(30 * time.Minute).Unix()})
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	cookieName := downloadCookieName(id)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: token, Path: a.cfg.BasePath + "/d/" + id, MaxAge: 30 * 60, HttpOnly: true, Secure: a.cfg.SecureCookies, SameSite: http.SameSiteLaxMode})
	_ = share
	http.Redirect(w, r, a.cfg.BasePath+"/d/"+id, http.StatusSeeOther)
}

func (a *App) handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	cookie, err := r.Cookie(downloadCookieName(id))
	if err != nil {
		http.Error(w, "download authorization required", http.StatusUnauthorized)
		return
	}
	token, ok := verifyToken(a.cfg.SigningKey, cookie.Value, "download", time.Now())
	wantIPKey := ipKey(a.cfg.IPHashKey, id, clientIP(r))
	if !ok || token.ShareID != id || token.IPKey != wantIPKey {
		http.Error(w, "download authorization expired", http.StatusUnauthorized)
		return
	}
	share, err := a.store.get(id)
	if err != nil || !share.ExpiresAt.After(time.Now()) {
		http.Error(w, "share unavailable", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": share.OriginalName}))
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("X-Accel-Redirect", "/_file_relay_internal/"+share.StoredName)
	w.WriteHeader(http.StatusOK)
}

func downloadCookieName(id string) string {
	if len(id) > 10 {
		id = id[:10]
	}
	return "relay_dl_" + id
}

func safeFilename(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, strings.TrimSpace(name))
	runes := []rune(name)
	if len(runes) > 180 {
		name = string(runes[:180])
	}
	return name
}

func (a *App) cleanupLoop() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		a.cleanupExpired()
	}
}

func (a *App) cleanupExpired() {
	removed, err := a.store.deleteExpired(time.Now())
	if err != nil {
		log.Printf("cleanup metadata: %v", err)
		return
	}
	for _, share := range removed {
		if err := os.Remove(filepath.Join(a.cfg.DataDir, share.StoredName)); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("cleanup %s: %v", share.ID, err)
		}
	}
	a.cleanupAbandonedUploads(time.Now().Add(-24 * time.Hour))
}

func (a *App) cleanupAbandonedUploads(olderThan time.Time) {
	a.uploadMu.Lock()
	defer a.uploadMu.Unlock()
	metadataFiles, err := filepath.Glob(filepath.Join(a.cfg.DataDir, ".upload-*.json"))
	if err != nil {
		log.Printf("scan abandoned uploads: %v", err)
		return
	}
	for _, metaPath := range metadataFiles {
		info, statErr := os.Stat(metaPath)
		if statErr != nil || !info.ModTime().Before(olderThan) {
			continue
		}
		base := strings.TrimSuffix(metaPath, ".json")
		_ = os.Remove(base + ".part")
		if err := os.Remove(metaPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("cleanup abandoned upload %s: %v", filepath.Base(base), err)
		}
	}
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for value := n / unit; value >= unit && exp < 5; value /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
