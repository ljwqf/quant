package cache

import (
	"sync"
	"time"
)

type CacheItem struct {
	value      interface{}
	expiration time.Time
}

type Cache struct {
	items map[string]*CacheItem
	mu    sync.RWMutex
}

func NewCache() *Cache {
	c := &Cache{
		items: make(map[string]*CacheItem),
	}
	go c.startCleanup()
	return c
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &CacheItem{
		value:      value,
		expiration: time.Now().Add(ttl),
	}
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	if time.Now().After(item.expiration) {
		return nil, false
	}

	return item.value, true
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*CacheItem)
}

func (c *Cache) Has(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return false
	}

	return !time.Now().After(item.expiration)
}

func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	now := time.Now()
	for _, item := range c.items {
		if now.Before(item.expiration) {
			count++
		}
	}
	return count
}

func (c *Cache) startCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		c.cleanup()
	}
}

func (c *Cache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiration) {
			delete(c.items, key)
		}
	}
}
