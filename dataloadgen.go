package dataloadgen

import (
	"sync"
	"time"
)

// GenericLoaderConfig captures the config to create a new GenericLoader
type GenericLoaderConfig[KeyT comparable, ValueT any] struct {
	// Fetch is a method that provides the data for the loader
	Fetch func(keys []KeyT) ([]ValueT, []error)

	// Wait is how long wait before sending a batch
	Wait time.Duration

	// MaxBatch will limit the maximum number of keys to send in one batch, 0 = not limit
	MaxBatch int
}

// NewGenericLoader creates a new GenreicLoader given a fetch, wait, and maxBatch
func NewGenericLoader[KeyT comparable, ValueT any](config GenericLoaderConfig[KeyT, ValueT]) *GenericLoader[KeyT, ValueT] {
	return &GenericLoader[KeyT, ValueT]{
		fetch:    config.Fetch,
		wait:     config.Wait,
		maxBatch: config.MaxBatch,
	}
}

// GenericLoader batches and caches requests
type GenericLoader[KeyT comparable, ValueT any] struct {
	// this method provides the data for the loader
	fetch func(keys []KeyT) ([]ValueT, []error)

	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	// INTERNAL

	// lazily created cache
	cache map[KeyT]ValueT

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *genericLoaderBatch[KeyT, ValueT]

	// mutex to prevent races
	mu sync.Mutex
}

type genericLoaderBatch[KeyT comparable, ValueT any] struct {
	keys    []KeyT
	data    []ValueT
	error   []error
	closing bool
	done    chan struct{}
}

// Load a ValueT by key, batching and caching will be applied automatically
func (l *GenericLoader[KeyT, ValueT]) Load(key KeyT) (ValueT, error) {
	return l.LoadThunk(key)()
}

// LoadThunk returns a function that when called will block waiting for a ValueT.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *GenericLoader[KeyT, ValueT]) LoadThunk(key KeyT) func() (ValueT, error) {
	l.mu.Lock()
	if it, ok := l.cache[key]; ok {
		l.mu.Unlock()
		return func() (ValueT, error) {
			return it, nil
		}
	}
	if l.batch == nil {
		l.batch = &genericLoaderBatch[KeyT, ValueT]{done: make(chan struct{})}
	}
	batch := l.batch
	pos := batch.keyIndex(l, key)
	l.mu.Unlock()

	return func() (ValueT, error) {
		<-batch.done

		var data ValueT
		if pos < len(batch.data) {
			data = batch.data[pos]
		}

		var err error
		// its convenient to be able to return a single error for everything
		if len(batch.error) == 1 {
			err = batch.error[0]
		} else if batch.error != nil {
			err = batch.error[pos]
		}

		if err == nil {
			l.mu.Lock()
			l.unsafeSet(key, data)
			l.mu.Unlock()
		}

		return data, err
	}
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *GenericLoader[KeyT, ValueT]) LoadAll(keys []KeyT) ([]ValueT, []error) {
	results := make([]func() (ValueT, error), len(keys))

	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}

	values := make([]ValueT, len(keys))
	errors := make([]error, len(keys))
	for i, thunk := range results {
		values[i], errors[i] = thunk()
	}
	return values, errors
}

// LoadAllThunk returns a function that when called will block waiting for a ValueT.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *GenericLoader[KeyT, ValueT]) LoadAllThunk(keys []KeyT) func() ([]ValueT, []error) {
	results := make([]func() (ValueT, error), len(keys))
	for i, key := range keys {
		results[i] = l.LoadThunk(key)
	}
	return func() ([]ValueT, []error) {
		values := make([]ValueT, len(keys))
		errors := make([]error, len(keys))
		for i, thunk := range results {
			values[i], errors[i] = thunk()
		}
		return values, errors
	}
}

// Prime the cache with the provided key and value. If the key already exists, no change is made
// and false is returned.
// (To forcefully prime the cache, clear the key first with loader.clear(key).prime(key, value).)
func (l *GenericLoader[KeyT, ValueT]) Prime(key KeyT, value ValueT) bool {
	l.mu.Lock()
	var found bool
	if _, found = l.cache[key]; !found {
		l.unsafeSet(key, value)
	}
	l.mu.Unlock()
	return !found
}

// Clear the value at key from the cache, if it exists
func (l *GenericLoader[KeyT, ValueT]) Clear(key KeyT) {
	l.mu.Lock()
	delete(l.cache, key)
	l.mu.Unlock()
}

func (l *GenericLoader[KeyT, ValueT]) unsafeSet(key KeyT, value ValueT) {
	if l.cache == nil {
		l.cache = map[KeyT]ValueT{}
	}
	l.cache[key] = value
}

// keyIndex will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (b *genericLoaderBatch[KeyT, ValueT]) keyIndex(l *GenericLoader[KeyT, ValueT], key KeyT) int {
	for i, existingKey := range b.keys {
		if key == existingKey {
			return i
		}
	}

	pos := len(b.keys)
	b.keys = append(b.keys, key)
	if pos == 0 {
		go b.startTimer(l)
	}

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		if !b.closing {
			b.closing = true
			l.batch = nil
			go b.end(l)
		}
	}

	return pos
}

func (b *genericLoaderBatch[KeyT, ValueT]) startTimer(l *GenericLoader[KeyT, ValueT]) {
	time.Sleep(l.wait)
	l.mu.Lock()

	// we must have hit a batch limit and are already finalizing this batch
	if b.closing {
		l.mu.Unlock()
		return
	}

	l.batch = nil
	l.mu.Unlock()

	b.end(l)
}

func (b *genericLoaderBatch[KeyT, ValueT]) end(l *GenericLoader[KeyT, ValueT]) {
	b.data, b.error = l.fetch(b.keys)
	close(b.done)
}
