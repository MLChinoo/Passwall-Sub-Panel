package passkey

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/go-webauthn/webauthn/webauthn"
)

const sessionTTL = 5 * time.Minute

// sessionStore holds the per-ceremony *webauthn.SessionData (the challenge +
// parameters) server-side between the Begin and Finish steps. It is SINGLE-USE:
// Take removes the entry, so a consumed challenge can't be replayed. The raw
// challenge deliberately never leaves the server (unlike the 2FA pending JWT).
type sessionStore struct {
	mu  sync.Mutex
	m   map[string]sessionEntry
	now func() time.Time
}

type sessionEntry struct {
	data    *webauthn.SessionData
	expires time.Time
}

func newSessionStore(now func() time.Time) *sessionStore {
	if now == nil {
		now = time.Now
	}
	return &sessionStore{m: make(map[string]sessionEntry), now: now}
}

// put stores the session data under a fresh random id and returns the id. It
// also opportunistically sweeps expired entries so abandoned ceremonies don't
// accumulate (Take is the normal removal path; this catches the ones never
// finished).
func (s *sessionStore) put(data *webauthn.SessionData) (string, error) {
	idBytes := make([]byte, 32)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	id := base64.RawURLEncoding.EncodeToString(idBytes)
	now := s.now()
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.m {
		if !e.expires.After(now) {
			delete(s.m, k)
		}
	}
	s.m[id] = sessionEntry{data: data, expires: now.Add(sessionTTL)}
	return id, nil
}

// take returns the session data for id and removes it (single-use). Returns nil
// if the id is unknown or already expired.
func (s *sessionStore) take(id string) *webauthn.SessionData {
	if id == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.m[id]
	if !ok {
		return nil
	}
	delete(s.m, id)
	if !e.expires.After(s.now()) {
		return nil
	}
	return e.data
}
