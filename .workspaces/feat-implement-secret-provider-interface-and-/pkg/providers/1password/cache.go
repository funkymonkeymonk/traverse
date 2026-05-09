package onepassword

import (
	"sync"
	"time"
)

// cacheEntry holds a cached secret with its expiration time
type cacheEntry struct {
	secret    *Secret
	expiresAt time.Time
}

// SecretCache provides thread-safe caching for secrets
type SecretCache struct {
	entries map[string]*cacheEntry
	ttl     time.Duration
	mu      sync.RWMutex
}

// NewSecretCache creates a new secret cache with the specified TTL
func NewSecretCache(ttl time.Duration) *SecretCache {
	return &SecretCache{
		entries: make(map[string]*cacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a secret from the cache
func (c *SecretCache) Get(path string) (*Secret, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[path]
	if !exists {
		return nil, false
	}

	// Check if entry has expired
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}

	return entry.secret, true
}

// Set stores a secret in the cache
func (c *SecretCache) Set(path string, secret *Secret) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[path] = &cacheEntry{
		secret:    secret,
		expiresAt: time.Now().Add(c.ttl),
	}
}

// Delete removes a secret from the cache
func (c *SecretCache) Delete(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, path)
}

// Clear removes all entries from the cache
func (c *SecretCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*cacheEntry)
}

// Cleanup removes expired entries from the cache
func (c *SecretCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for path, entry := range c.entries {
		if now.After(entry.expiresAt) {
			delete(c.entries, path)
		}
	}
}

// Size returns the number of entries in the cache
func (c *SecretCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.entries)
}
