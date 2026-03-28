package web

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"remnafamily/internal/config"

	"github.com/redis/go-redis/v9"
)

var errStateNotFound = errors.New("state not found")

type stateStore interface {
	SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	GetJSON(ctx context.Context, key string, dest interface{}) error
	Delete(ctx context.Context, key string) error
}

func newStateStore(cfg config.ValkeyConfig) stateStore {
	if cfg.Addr == "" {
		return newMemoryStateStore()
	}

	options := &redis.Options{
		Password: cfg.Password,
		DB:       cfg.DB,
	}

	switch {
	case strings.HasPrefix(cfg.Addr, "unix://"):
		options.Network = "unix"
		options.Addr = strings.TrimPrefix(cfg.Addr, "unix://")
	case strings.HasPrefix(cfg.Addr, "/"):
		options.Network = "unix"
		options.Addr = cfg.Addr
	default:
		options.Addr = cfg.Addr
	}

	client := redis.NewClient(options)

	return &valkeyStateStore{client: client}
}

type memoryStateEntry struct {
	data      []byte
	expiresAt time.Time
}

type memoryStateStore struct {
	mu      sync.RWMutex
	entries map[string]memoryStateEntry
}

func newMemoryStateStore() *memoryStateStore {
	store := &memoryStateStore{
		entries: make(map[string]memoryStateEntry),
	}
	go store.cleanupLoop()
	return store
}

func (s *memoryStateStore) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[key] = memoryStateEntry{
		data:      payload,
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (s *memoryStateStore) GetJSON(ctx context.Context, key string, dest interface{}) error {
	s.mu.RLock()
	entry, ok := s.entries[key]
	s.mu.RUnlock()
	if !ok {
		return errStateNotFound
	}
	if time.Now().After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.entries, key)
		s.mu.Unlock()
		return errStateNotFound
	}
	return json.Unmarshal(entry.data, dest)
}

func (s *memoryStateStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, key)
	return nil
}

func (s *memoryStateStore) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for key, entry := range s.entries {
			if now.After(entry.expiresAt) {
				delete(s.entries, key)
			}
		}
		s.mu.Unlock()
	}
}

type valkeyStateStore struct {
	client *redis.Client
}

func (s *valkeyStateStore) SetJSON(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, payload, ttl).Err()
}

func (s *valkeyStateStore) GetJSON(ctx context.Context, key string, dest interface{}) error {
	payload, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errStateNotFound
		}
		return err
	}
	return json.Unmarshal(payload, dest)
}

func (s *valkeyStateStore) Delete(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}
