package utils

import (
	"sync"
)

// Promise helps to Resolve or Reject only once.
type Promise struct {
	lock sync.Mutex
	wait chan struct{}

	done bool
	val  interface{}
	err  error
}

// NewPromise instance
func NewPromise() *Promise {
	return &Promise{wait: make(chan struct{})}
}

// Wait returns a channel which will be closed once the Done is called.
func (p *Promise) Wait() <-chan struct{} {
	return p.wait
}

// Done results the promise with specified val and err.
func (p *Promise) Done(val interface{}, err error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.done {
		return
	}
	p.done = true
	p.val = val
	p.err = err
	close(p.wait)
}

// Result of the Done
func (p *Promise) Result() (interface{}, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.val, p.err
}
