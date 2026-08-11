package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var (
	errShareNotFound = errors.New("share not found")
	errShareExpired  = errors.New("share expired")
	errLocked        = errors.New("too many password attempts")
	errBadPassword   = errors.New("invalid password")
	errDownloadLimit = errors.New("download limit reached")
)

type AttemptState struct {
	Failures    int       `json:"failures"`
	WindowStart time.Time `json:"window_start"`
	LockedUntil time.Time `json:"locked_until,omitempty"`
}

type Share struct {
	ID           string                  `json:"id"`
	OriginalName string                  `json:"original_name"`
	StoredName   string                  `json:"stored_name"`
	Size         int64                   `json:"size"`
	SHA256       string                  `json:"sha256"`
	CreatedAt    time.Time               `json:"created_at"`
	ExpiresAt    time.Time               `json:"expires_at"`
	PasswordHash string                  `json:"password_hash,omitempty"`
	Attempts     map[string]AttemptState `json:"attempts,omitempty"`
	Downloads    map[string]int          `json:"downloads,omitempty"`
}

type persistedState struct {
	Version int               `json:"version"`
	Shares  map[string]*Share `json:"shares"`
}

type Store struct {
	mu   sync.Mutex
	path string
	data persistedState
}

func openStore(path string) (*Store, error) {
	s := &Store{path: path, data: persistedState{Version: 1, Shares: make(map[string]*Share)}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, err
	}
	if s.data.Shares == nil {
		s.data.Shares = make(map[string]*Share)
	}
	return s, nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func (s *Store) create(share *Share) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data.Shares[share.ID]; exists {
		return errors.New("share already exists")
	}
	if share.Attempts == nil {
		share.Attempts = make(map[string]AttemptState)
	}
	if share.Downloads == nil {
		share.Downloads = make(map[string]int)
	}
	s.data.Shares[share.ID] = share
	if err := s.saveLocked(); err != nil {
		delete(s.data.Shares, share.ID)
		return err
	}
	return nil
}

func cloneShare(in *Share) Share {
	out := *in
	out.Attempts = make(map[string]AttemptState, len(in.Attempts))
	for k, v := range in.Attempts {
		out.Attempts[k] = v
	}
	out.Downloads = make(map[string]int, len(in.Downloads))
	for k, v := range in.Downloads {
		out.Downloads[k] = v
	}
	return out
}

func (s *Store) get(id string) (Share, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	share, ok := s.data.Shares[id]
	if !ok {
		return Share{}, errShareNotFound
	}
	return cloneShare(share), nil
}

func (s *Store) list() []Share {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Share, 0, len(s.data.Shares))
	for _, share := range s.data.Shares {
		items = append(items, cloneShare(share))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt.After(items[j].CreatedAt) })
	return items
}

func (s *Store) authorize(id, key, password string, now time.Time, maxAttempts int, window, lockout time.Duration, maxDownloads int) (Share, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	share, ok := s.data.Shares[id]
	if !ok {
		return Share{}, errShareNotFound
	}
	if !share.ExpiresAt.After(now) {
		return Share{}, errShareExpired
	}
	if share.Attempts == nil {
		share.Attempts = make(map[string]AttemptState)
	}
	if share.Downloads == nil {
		share.Downloads = make(map[string]int)
	}
	attempt := share.Attempts[key]
	if attempt.LockedUntil.After(now) {
		return Share{}, errLocked
	}
	if !attempt.WindowStart.IsZero() && now.Sub(attempt.WindowStart) >= window {
		attempt = AttemptState{}
	}
	if share.PasswordHash != "" && !verifyPassword(share.PasswordHash, password) {
		if attempt.WindowStart.IsZero() {
			attempt.WindowStart = now
		}
		attempt.Failures++
		if attempt.Failures >= maxAttempts {
			attempt.LockedUntil = now.Add(lockout)
		}
		share.Attempts[key] = attempt
		_ = s.saveLocked()
		if attempt.LockedUntil.After(now) {
			return Share{}, errLocked
		}
		return Share{}, errBadPassword
	}
	if share.Downloads[key] >= maxDownloads {
		return Share{}, errDownloadLimit
	}
	delete(share.Attempts, key)
	share.Downloads[key]++
	if err := s.saveLocked(); err != nil {
		share.Downloads[key]--
		return Share{}, err
	}
	return cloneShare(share), nil
}

func (s *Store) delete(id string) (Share, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	share, ok := s.data.Shares[id]
	if !ok {
		return Share{}, errShareNotFound
	}
	copy := cloneShare(share)
	delete(s.data.Shares, id)
	if err := s.saveLocked(); err != nil {
		s.data.Shares[id] = share
		return Share{}, err
	}
	return copy, nil
}

func (s *Store) deleteExpired(now time.Time) ([]Share, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []Share
	for id, share := range s.data.Shares {
		if !share.ExpiresAt.After(now) {
			removed = append(removed, cloneShare(share))
			delete(s.data.Shares, id)
		}
	}
	if len(removed) == 0 {
		return nil, nil
	}
	if err := s.saveLocked(); err != nil {
		for i := range removed {
			copy := removed[i]
			s.data.Shares[copy.ID] = &copy
		}
		return nil, err
	}
	return removed, nil
}
