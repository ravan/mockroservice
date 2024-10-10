package util

import "sync"

type Counter struct {
	mu        sync.Mutex
	counter   int
	TriggerOn int
	Active    bool
}

func (c *Counter) Reset() {
	c.mu.Lock()
	c.counter = 0 // Reset after error
	c.mu.Unlock()
}

func (c *Counter) Increment() {
	c.mu.Lock()
	c.counter++
	if c.counter >= 1000 {
		c.counter = 1
	}
	c.mu.Unlock()
}

func (c *Counter) GetCount() int {
	c.mu.Lock()
	count := c.counter
	c.mu.Unlock()
	return count
}

func (c *Counter) ShouldTrigger() bool {
	c.mu.Lock()
	r := c.Active && c.counter >= c.TriggerOn-1
	c.mu.Unlock()
	return r
}
