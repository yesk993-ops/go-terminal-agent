package cache

import (
	"container/list"
	"sync"
	"time"

	"github.com/agent/ai-terminal/internal/core"
)

type entry struct {
	key   string
	value string
	exp   time.Time
}

type LRUCache struct {
	mu       sync.RWMutex
	items    map[string]*list.Element
	order    *list.List
	maxSize  int
	defaultTTL time.Duration
}

func New(maxSize int, defaultTTL time.Duration) *LRUCache {
	if maxSize <= 0 {
		maxSize = 500
	}
	return &LRUCache{
		items:      make(map[string]*list.Element),
		order:      list.New(),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}
}

func (c *LRUCache) Get(key string) (string, bool) {
	c.mu.RLock()
	elem, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	elem = c.items[key]
	if elem == nil {
		return "", false
	}

	ent := elem.Value.(*entry)
	if !ent.exp.IsZero() && time.Now().After(ent.exp) {
		c.removeElement(elem)
		return "", false
	}

	c.order.MoveToFront(elem)
	return ent.value, true
}

func (c *LRUCache) Set(key string, value string, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		ent := elem.Value.(*entry)
		ent.value = value
		ent.exp = time.Now().Add(ttl)
		return
	}

	ent := &entry{
		key:   key,
		value: value,
		exp:   time.Now().Add(ttl),
	}
	elem := c.order.PushFront(ent)
	c.items[key] = elem

	if c.order.Len() > c.maxSize {
		c.removeOldest()
	}
}

func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.removeElement(elem)
	}
}

func (c *LRUCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.order.Init()
}

func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

func (c *LRUCache) removeElement(elem *list.Element) {
	c.order.Remove(elem)
	ent := elem.Value.(*entry)
	delete(c.items, ent.key)
}

func (c *LRUCache) removeOldest() {
	elem := c.order.Back()
	if elem != nil {
		c.removeElement(elem)
	}
}

var _ core.Cache = (*LRUCache)(nil)
