package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CacheEntry represents a cache entry mapping hash to digest
type CacheEntry struct {
	Hash   string `json:"hash"`
	Digest string `json:"digest"`
}

// Cache manages the cache index
type Cache struct {
	store *Store
}

// NewCache creates a new Cache instance
func NewCache(store *Store) *Cache {
	return &Cache{store: store}
}

// GetCachedDigest looks up a cached digest by hash
func (c *Cache) GetCachedDigest(hash string) (string, bool) {
	entries, err := c.loadCache()
	if err != nil {
		return "", false
	}

	if entry, exists := entries[hash]; exists {
		return entry.Digest, true
	}

	return "", false
}

// SetCachedDigest stores a digest for a given hash
func (c *Cache) SetCachedDigest(hash, digest string) error {
	entries, err := c.loadCache()
	if err != nil {
		entries = make(map[string]CacheEntry)
	}

	entries[hash] = CacheEntry{
		Hash:   hash,
		Digest: digest,
	}

	return c.saveCache(entries)
}

// loadCache loads the cache index from disk
func (c *Cache) loadCache() (map[string]CacheEntry, error) {
	path := filepath.Join(c.store.rootPath, ".dockstep", CacheFile)

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]CacheEntry), nil
		}
		return nil, err
	}
	defer file.Close()

	var entries map[string]CacheEntry
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("failed to decode cache: %w", err)
	}

	return entries, nil
}

// saveCache saves the cache index to disk
func (c *Cache) saveCache(entries map[string]CacheEntry) error {
	path := filepath.Join(c.store.rootPath, ".dockstep", CacheFile)

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(entries)
}

// ClearCache removes all cache entries
func (c *Cache) ClearCache() error {
	path := filepath.Join(c.store.rootPath, ".dockstep", CacheFile)
	return os.Remove(path)
}

// GetCacheStats returns cache statistics
func (c *Cache) GetCacheStats() (int, error) {
	entries, err := c.loadCache()
	if err != nil {
		return 0, err
	}
	return len(entries), nil
}
