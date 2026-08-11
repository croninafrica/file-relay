package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory  = 32 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
)

func hashPassword(password string) (string, error) {
	if len(password) < 12 {
		return "", errors.New("password must be at least 12 characters")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" || parts[2] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return false
	}
	if memory > 128*1024 || iterations > 10 || threads > 8 || memory < 8*1024 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) < 16 || len(want) > 64 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, iterations, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

func randomToken(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type signedToken struct {
	Kind    string `json:"k"`
	ShareID string `json:"s,omitempty"`
	IPKey   string `json:"i,omitempty"`
	CSRF    string `json:"c,omitempty"`
	Expires int64  `json:"e"`
}

func signToken(key []byte, payload signedToken) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(raw) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func verifyToken(key []byte, value, kind string, now time.Time) (signedToken, bool) {
	var payload signedToken
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return payload, false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return payload, false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return payload, false
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(raw)
	if !hmac.Equal(sig, mac.Sum(nil)) || json.Unmarshal(raw, &payload) != nil {
		return signedToken{}, false
	}
	if payload.Kind != kind || payload.Expires <= now.Unix() {
		return signedToken{}, false
	}
	return payload, true
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	remote := net.ParseIP(strings.TrimSpace(host))
	if remote != nil && remote.IsLoopback() {
		if forwarded := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); forwarded != nil {
			return forwarded.String()
		}
	}
	if remote == nil {
		return "unknown"
	}
	return remote.String()
}

func ipKey(secret []byte, shareID, ip string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(shareID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(ip))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:18])
}

type loginLimiter struct {
	attempts map[string][]time.Time
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{attempts: make(map[string][]time.Time)}
}

func (l *loginLimiter) allow(ip string, now time.Time) bool {
	cutoff := now.Add(-15 * time.Minute)
	items := l.attempts[ip][:0]
	for _, t := range l.attempts[ip] {
		if t.After(cutoff) {
			items = append(items, t)
		}
	}
	l.attempts[ip] = items
	return len(items) < 5
}

func (l *loginLimiter) fail(ip string, now time.Time) { l.attempts[ip] = append(l.attempts[ip], now) }
func (l *loginLimiter) clear(ip string)               { delete(l.attempts, ip) }

func parsePositiveInt(value string, fallback int) int {
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
