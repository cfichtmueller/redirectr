package cache

import (
	"testing"
	"time"
)

func TestLRUCache(t *testing.T) {
	cache := NewLRUCache(2)

	// Test basic put and get
	cache.Put("test1", &CacheEntry{TargetDomain: "target1", CachedAt: time.Now()})
	if entry, found := cache.Get("test1"); !found || entry.TargetDomain != "target1" {
		t.Errorf("Expected to find test1, got found=%v, target=%s", found, entry.TargetDomain)
	}

	// Test cache capacity
	cache.Put("test2", &CacheEntry{TargetDomain: "target2", CachedAt: time.Now()})
	cache.Put("test3", &CacheEntry{TargetDomain: "target3", CachedAt: time.Now()})

	// test1 should be evicted (LRU)
	if _, found := cache.Get("test1"); found {
		t.Error("Expected test1 to be evicted")
	}

	// test2 and test3 should still be there
	if _, found := cache.Get("test2"); !found {
		t.Error("Expected test2 to still be in cache")
	}
	if _, found := cache.Get("test3"); !found {
		t.Error("Expected test3 to still be in cache")
	}

	// Test cache size
	if cache.Size() != 2 {
		t.Errorf("Expected cache size 2, got %d", cache.Size())
	}
}
