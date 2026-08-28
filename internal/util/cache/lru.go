package cache

import (
	"sync"
	"time"
)

// CacheEntry represents a cached redirect entry
type CacheEntry struct {
	TargetDomain string
	RedirectID   string
	UserID       string
	RedirectType string
	UTMTags      interface{} // Will be *redirect.UTMTags but using interface{} to avoid circular import
	CachedAt     time.Time
}

// LRUCache implements a thread-safe LRU cache for redirect lookups
type LRUCache struct {
	capacity int
	cache    map[string]*Node
	head     *Node
	tail     *Node
	mutex    sync.RWMutex
}

// Node represents a node in the doubly linked list
type Node struct {
	key   string
	value *CacheEntry
	prev  *Node
	next  *Node
}

// NewLRUCache creates a new LRU cache with the specified capacity
func NewLRUCache(capacity int) *LRUCache {
	cache := &LRUCache{
		capacity: capacity,
		cache:    make(map[string]*Node),
	}

	// Initialize dummy head and tail nodes
	cache.head = &Node{}
	cache.tail = &Node{}
	cache.head.next = cache.tail
	cache.tail.prev = cache.head

	return cache
}

func (c *LRUCache) GetCapacity() int {
	return c.capacity
}

// Get retrieves a value from the cache and moves it to the front
func (c *LRUCache) Get(key string) (*CacheEntry, bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if node, exists := c.cache[key]; exists {
		// Move to front (most recently used)
		c.moveToFront(node)
		return node.value, true
	}
	return nil, false
}

// Put stores a value in the cache
func (c *LRUCache) Put(key string, value *CacheEntry) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if node, exists := c.cache[key]; exists {
		// Update existing node
		node.value = value
		c.moveToFront(node)
		return
	}

	// Add new node
	node := &Node{
		key:   key,
		value: value,
	}
	c.cache[key] = node
	c.addToFront(node)

	// Remove least recently used if capacity exceeded
	if len(c.cache) > c.capacity {
		c.removeTail()
	}
}

// Delete removes a key from the cache
func (c *LRUCache) Delete(key string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if node, exists := c.cache[key]; exists {
		c.removeNode(node)
		delete(c.cache, key)
	}
}

// Clear removes all entries from the cache
func (c *LRUCache) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.cache = make(map[string]*Node)
	c.head.next = c.tail
	c.tail.prev = c.head
}

// Size returns the current number of entries in the cache
func (c *LRUCache) Size() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.cache)
}

// moveToFront moves a node to the front of the list
func (c *LRUCache) moveToFront(node *Node) {
	c.removeNode(node)
	c.addToFront(node)
}

// addToFront adds a node to the front of the list
func (c *LRUCache) addToFront(node *Node) {
	node.prev = c.head
	node.next = c.head.next
	c.head.next.prev = node
	c.head.next = node
}

// removeNode removes a node from the list
func (c *LRUCache) removeNode(node *Node) {
	node.prev.next = node.next
	node.next.prev = node.prev
}

// removeTail removes the least recently used node
func (c *LRUCache) removeTail() {
	if c.tail.prev != c.head {
		node := c.tail.prev
		c.removeNode(node)
		delete(c.cache, node.key)
	}
}
