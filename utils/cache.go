package utils

import (
	"sync"
)

type CacheBase[K comparable, V any] interface {
	Add(K, V)
	Get(K) (V, bool)
}

type Cache[K comparable, V any] interface {
	CacheBase[K, V]

	RemoveAccessed()
}

// Map object cache
type cache[K comparable, V any] struct {
	cacheMap map[K]V
	accessed []K

	sync.RWMutex
}

func NewCache[K comparable, V any]() Cache[K, V] {
	return &cache[K, V]{
		cacheMap: make(map[K]V),
		accessed: nil,
	}
}

func (c *cache[K, V]) Add(k K, v V) {
	c.Lock()
	defer c.Unlock()

	c.cacheMap[k] = v
}

func (c *cache[K, V]) Get(k K) (V, bool) {
	c.Lock()
	defer c.Unlock()

	v, ok := c.cacheMap[k]
	if ok {
		c.accessed = append(c.accessed, k)
	}
	return v, ok
}

func (c *cache[K, V]) RemoveAccessed() {
	c.Lock()
	defer c.Unlock()

	for _, k := range c.accessed {
		delete(c.cacheMap, k)
	}
	c.accessed = nil
}
