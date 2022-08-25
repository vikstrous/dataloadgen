package dataloadgen

import (
	"fmt"
	"sync"
	"time"
)

// Option allows for configuration of loader fields.
type Option func(*loaderConfig)

// WithBatchCapacity sets the batch capacity. Default is 0 (unbounded)
func WithBatchCapacity(c int) Option {
	return func(l *loaderConfig) {
		l.maxBatch = c
	}
}

// WithWait sets the amount of time to wait before triggering a batch.
// Default duration is 16 milliseconds.
func WithWait(d time.Duration) Option {
	return func(l *loaderConfig) {
		l.wait = d
	}
}

// NewLoader creates a new GenericLoader given a fetch, wait, and maxBatch
func NewLoader[KeyT comparable, ValueT any](fetch func(keys []KeyT) ([]ValueT, []error), options ...Option) *Loader[KeyT, ValueT] {
	config := &loaderConfig{
		wait:     16 * time.Millisecond,
		maxBatch: 0, //unlimited
	}
	for _, o := range options {
		o(config)
	}
	l := &Loader[KeyT, ValueT]{
		fetch:        fetch,
		loaderConfig: config,
		cache:        map[KeyT]func() (ValueT, error){},
	}
	return l
}

type loaderConfig struct {
	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int
}

// Loader batches and caches requests
type Loader[KeyT comparable, ValueT any] struct {
	// this method provides the data for the loader
	fetch func(keys []KeyT) ([]ValueT, []error)

	*loaderConfig

	// INTERNAL

	// lazily created cache
	cache map[KeyT]func() (ValueT, error)

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *loaderBatch[KeyT, ValueT]

	// mutex to prevent races
	mu sync.Mutex
}

type loaderBatch[KeyT comparable, ValueT any] struct {
	keys    []KeyT
	data    []ValueT
	error   []error
	closing bool
	done    chan struct{}
}

// Load a ValueT by key, batching and caching will be applied automatically
func (l *Loader[KeyT, ValueT]) Load(key KeyT) (ValueT, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a ValueT.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *Loader[KeyT, ValueT]) LoadThunk(key KeyT) func() (ValueT, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if it, ok := l.cache[key]; ok {
		return it
	}
	if l.batch == nil {
		l.batch = &loaderBatch[KeyT, ValueT]{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)

	thunk := func() (ValueT, error) {
		<-batch.done

		var data ValueT

		// Return early if there's a single error and it's not nil
		if len(batch.error) == 1 && batch.error[0] != nil {
			return data, batch.error[0]
		}

		// If the batch function returned the wrong number of responses, return an error to all callers
		if len(batch.data) != len(batch.keys) {
			return data, fmt.Errorf("bug in loader: %d values returned for %d keys", len(batch.data), len(batch.keys))
		}

		if pos < len(batch.data) {
			data = batch.data[pos]
		}

		var err error
		if batch.error != nil {
			err = batch.error[pos]
		}

		return data, err
	}
	l.cache[key] = thunk
	return thunk
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *Loader[KeyT, ValueT]) LoadAll(keys []KeyT) ([]ValueT, []error) {
	thunks := make([]func() (ValueT, error), len(keys))

	for i, key := range keys {
		thunks[i] = l.LoadThunk(key)
	}

	values := make([]ValueT, len(keys))
	errors := make([]error, len(keys))
	allNil := true
	for i, thunk := range thunks {
		values[i], errors[i] = thunk()
		if errors[i] != nil {
			allNil = false
		}
	}
	if allNil {
		return values, nil
	}
	return values, errors
}

// LoadAllThunk returns a function that when called will block waiting for a ValueT.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *Loader[KeyT, ValueT]) LoadAllThunk(keys []KeyT) func() ([]ValueT, []error) {
	thunks := make([]func() (ValueT, error), len(keys))
	for i, key := range keys {
		thunks[i] = l.LoadThunk(key)
	}
	return func() ([]ValueT, []error) {
		values := make([]ValueT, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range thunks {
			values[i], errors[i] = thunk()
		}
		return values, errors
	}
}

// Prime the cache with the provided key and value. If the key already exists, no change is made
// and false is returned.
// (To forcefully prime the cache, clear the key first with loader.Clear(key).Prime(key, value).)
func (l *Loader[KeyT, ValueT]) Prime(key KeyT, value ValueT) bool {
	l.mu.Lock()
	var found bool
	if _, found = l.cache[key]; !found {
		l.cache[key] = func() (ValueT, error) { return value, nil }
	}
	l.mu.Unlock()
	return !found
}

// Clear the value at key from the cache, if it exists
func (l *Loader[KeyT, ValueT]) Clear(key KeyT) {
	l.mu.Lock()
	delete(l.cache, key)
	l.mu.Unlock()
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *loaderBatch[KeyT, ValueT]) keyIndex(l *Loader[KeyT, ValueT], key KeyT) int {
	for i, existingKey := range b.keys {
		if key == existingKey {
			return i
		}
	}

	pos := len(b.keys)
	b.keys = append(b.keys, key)
	if pos == 0 {
		go func(l *Loader[KeyT, ValueT]) {
			time.Sleep(l.wait)
			l.mu.Lock()

			// we must have hit a batch limit and are already finalizing this batch
			if b.closing {
				l.mu.Unlock()
				return
			}

			l.batch = nil
			l.mu.Unlock()

			b.data, b.error = l.fetch(b.keys)
			close(b.done)
		}(l)
	}

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		if !b.closing {
			b.closing = true
			l.batch = nil
			go func(l *Loader[KeyT, ValueT]) {
				b.data, b.error = l.fetch(b.keys)
				close(b.done)
			}(l)
		}
	}

	return pos
}
