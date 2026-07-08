package cache

import (
	"fmt"
	"testing"
	"time"
)

func TestLRUCacheGetSet(t *testing.T) {
	c := New(10, time.Minute)

	c.Set("key1", "value1", 0)
	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if val != "value1" {
		t.Fatalf("expected 'value1', got %q", val)
	}
}

func TestLRUCacheMiss(t *testing.T) {
	c := New(10, time.Minute)
	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected nonexistent key to not be found")
	}
}

func TestLRUCacheEviction(t *testing.T) {
	c := New(2, time.Minute)

	c.Set("a", "1", 0)
	c.Set("b", "2", 0)
	c.Set("c", "3", 0)

	if c.Len() != 2 {
		t.Fatalf("expected len 2, got %d", c.Len())
	}

	_, ok := c.Get("a")
	if ok {
		t.Fatal("expected 'a' to be evicted")
	}

	val, ok := c.Get("b")
	if !ok || val != "2" {
		t.Fatalf("expected 'b'=2, got %q, %v", val, ok)
	}

	val, ok = c.Get("c")
	if !ok || val != "3" {
		t.Fatalf("expected 'c'=3, got %q, %v", val, ok)
	}
}

func TestLRUCacheTTL(t *testing.T) {
	c := New(10, 50*time.Millisecond)

	c.Set("key", "value", 50*time.Millisecond)
	time.Sleep(60 * time.Millisecond)

	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected key to expire")
	}
}

func TestLRUCacheDelete(t *testing.T) {
	c := New(10, time.Minute)

	c.Set("key", "value", 0)
	c.Delete("key")

	_, ok := c.Get("key")
	if ok {
		t.Fatal("expected key to be deleted")
	}
}

func TestLRUCacheClear(t *testing.T) {
	c := New(10, time.Minute)

	c.Set("a", "1", 0)
	c.Set("b", "2", 0)
	c.Clear()

	if c.Len() != 0 {
		t.Fatalf("expected len 0 after clear, got %d", c.Len())
	}
}

func TestLRUCacheUpdate(t *testing.T) {
	c := New(10, time.Minute)

	c.Set("key", "old", 0)
	c.Set("key", "new", 0)

	val, ok := c.Get("key")
	if !ok || val != "new" {
		t.Fatalf("expected 'new', got %q", val)
	}
}

func TestLRUCacheConcurrent(t *testing.T) {
	c := New(100, time.Minute)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				c.Set("key", "value", 0)
				c.Get("key")
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestLRUCacheConcurrentGetDelete(t *testing.T) {
	c := New(100, time.Minute)

	for i := 0; i < 50; i++ {
		c.Set(fmt.Sprintf("k%d", i), fmt.Sprintf("v%d", i), 0)
	}

	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				c.Get(fmt.Sprintf("k%d", j))
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				c.Delete(fmt.Sprintf("k%d", j))
			}
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}

func TestLRUCacheConcurrentSetClear(t *testing.T) {
	c := New(50, time.Minute)

	done := make(chan bool, 20)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				c.Set(fmt.Sprintf("k%d", j%50), "v", 0)
				c.Get(fmt.Sprintf("k%d", j%50))
			}
			done <- true
		}()
	}
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				c.Clear()
			}
			done <- true
		}()
	}

	for i := 0; i < 20; i++ {
		<-done
	}
}
