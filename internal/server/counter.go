package server

import "sync"

type Counter struct {
	mu           sync.Mutex
	counter      int
	errorAfter   int
	errorEnabled bool
}

func (c *Counter) Reset() {
	c.mu.Lock()
	c.counter = 0 // Reset after error
	c.mu.Unlock()
}

func (c *Counter) Increment() {
	c.mu.Lock()
	c.counter++
	c.mu.Unlock()
}

func (c *Counter) GetCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counter
}

func (c *Counter) ShouldError() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.errorEnabled && c.counter >= c.errorAfter-1
}
