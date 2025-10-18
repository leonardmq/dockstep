package store

import (
	"testing"
)

func TestCache(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	store.Init()
	cache := NewCache(store)

	// Test SetCachedDigest
	hash := "test-hash-123"
	digest := "sha256:test-digest-456"

	if err := cache.SetCachedDigest(hash, digest); err != nil {
		t.Fatalf("Failed to set cached digest: %v", err)
	}

	// Test GetCachedDigest
	retrieved, exists := cache.GetCachedDigest(hash)
	if !exists {
		t.Error("Expected cached digest to exist")
	}

	if retrieved != digest {
		t.Errorf("Expected digest %s, got %s", digest, retrieved)
	}

	// Test non-existent hash
	_, exists = cache.GetCachedDigest("non-existent")
	if exists {
		t.Error("Expected non-existent hash to not exist")
	}
}

func TestCacheStats(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	store.Init()
	cache := NewCache(store)

	// Test empty cache
	count, err := cache.GetCacheStats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 cache entries, got %d", count)
	}

	// Add some entries
	cache.SetCachedDigest("hash1", "digest1")
	cache.SetCachedDigest("hash2", "digest2")

	count, err = cache.GetCacheStats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}

	if count != 2 {
		t.Errorf("Expected 2 cache entries, got %d", count)
	}
}

func TestClearCache(t *testing.T) {
	tmpDir := t.TempDir()
	store := New(tmpDir)
	store.Init()
	cache := NewCache(store)

	// Add some entries
	cache.SetCachedDigest("hash1", "digest1")
	cache.SetCachedDigest("hash2", "digest2")

	// Clear cache
	if err := cache.ClearCache(); err != nil {
		t.Fatalf("Failed to clear cache: %v", err)
	}

	// Check that entries are gone
	count, err := cache.GetCacheStats()
	if err != nil {
		t.Fatalf("Failed to get cache stats: %v", err)
	}

	if count != 0 {
		t.Errorf("Expected 0 cache entries after clear, got %d", count)
	}
}
