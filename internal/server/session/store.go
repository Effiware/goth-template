// Package session wraps a gorilla/sessions backend: the session ID lives in the
// cookie, the values live in Redis under KeyPrefix. Swap the backend (see
// NewStoreWithBackend) to run without Redis.
package session

import (
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
	"github.com/rbcervilla/redisstore/v9"
	"github.com/redis/go-redis/v9"
)

// SessionName is the cookie name of the app session.
const SessionName = "goth-template-session"

// DefaultKeyPrefix namespaces session keys in Redis.
const DefaultKeyPrefix = "sess:"

func init() {
	// Values are gob-encoded; register any non-primitive type you store.
	gob.Register(time.Time{})
	gob.Register([]string{})
}

// Store is the session backend plus the cookie it reads/writes.
type Store struct {
	store sessions.Store
	name  string
	// rdb + keyPrefix enable ID rotation in Rotate; nil/"" on non-Redis backends,
	// where rotation degrades to blanking the ID.
	rdb       redis.UniversalClient
	keyPrefix string
}

// StoreConfig holds configuration for creating the session store.
type StoreConfig struct {
	MaxAge     int
	Secure     bool
	RedisURL   string
	CookieName string // default: SessionName
	KeyPrefix  string // default: DefaultKeyPrefix
}

// NewStore creates a Redis-backed session store.
func NewStore(ctx context.Context, cfg StoreConfig) (*Store, error) {
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse redis URL: %w", err)
	}

	client := redis.NewClient(opts)
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	rs, err := redisstore.NewRedisStore(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis store: %w", err)
	}

	keyPrefix := cfg.KeyPrefix
	if keyPrefix == "" {
		keyPrefix = DefaultKeyPrefix
	}
	name := cfg.CookieName
	if name == "" {
		name = SessionName
	}

	rs.KeyPrefix(keyPrefix)
	rs.Options(sessions.Options{
		Path:     "/",
		MaxAge:   cfg.MaxAge,
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: http.SameSiteLaxMode,
	})

	return &Store{store: rs, name: name, rdb: client, keyPrefix: keyPrefix}, nil
}

// NewStoreWithBackend wraps an arbitrary gorilla/sessions backend — a
// CookieStore when no Redis is configured, or a test double.
func NewStoreWithBackend(backend sessions.Store, name string) *Store {
	if name == "" {
		name = SessionName
	}
	return &Store{store: backend, name: name}
}

// Get returns a named session; Session returns this store's own.
func (s *Store) Get(r *http.Request, name string) (*sessions.Session, error) {
	return s.store.Get(r, name)
}

func (s *Store) Session(r *http.Request) (*sessions.Session, error) {
	return s.store.Get(r, s.name)
}

// Set writes values into the session in a single backend round-trip.
func (s *Store) Set(w http.ResponseWriter, r *http.Request, values map[string]any) error {
	sess, err := s.Session(r)
	if err != nil {
		return err
	}
	for k, v := range values {
		sess.Values[k] = v
	}
	return sess.Save(r, w)
}

// Value returns a raw session value; ok is false when absent or unreadable.
func (s *Store) Value(r *http.Request, key string) (any, bool) {
	sess, err := s.Session(r)
	if err != nil {
		return nil, false
	}
	v, ok := sess.Values[key]
	return v, ok
}

// Delete removes keys from the session.
func (s *Store) Delete(w http.ResponseWriter, r *http.Request, keys ...string) error {
	sess, err := s.Session(r)
	if err != nil {
		return err
	}
	for _, k := range keys {
		delete(sess.Values, k)
	}
	return sess.Save(r, w)
}

// Rotate mints a fresh session ID, carrying the values over. Call it on any
// privilege change (login): redisstore keys on the cookie value verbatim, so
// without rotation a pre-login ID survives as a fixation vector.
func (s *Store) Rotate(w http.ResponseWriter, r *http.Request) error {
	sess, err := s.Session(r)
	if err != nil {
		return err
	}
	if sess.ID != "" {
		if s.rdb != nil {
			if err := s.rdb.Del(r.Context(), s.keyPrefix+sess.ID).Err(); err != nil {
				slog.WarnContext(r.Context(), "session: failed to drop old key on rotation", "error", err)
			}
		}
		sess.ID = "" // Save mints a new ID; Values carry over
	}
	return sess.Save(r, w)
}

// Clear drops every value and expires the cookie.
func (s *Store) Clear(w http.ResponseWriter, r *http.Request) error {
	sess, err := s.Session(r)
	if err != nil {
		sess, _ = s.store.New(r, s.name)
	}

	sess.Values = make(map[any]any)
	sess.Options.MaxAge = -1

	return sess.Save(r, w)
}
