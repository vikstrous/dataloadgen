package dataloadgen

import (
	"context"
	"fmt"
	"sync"
	"time"

	trace "go.opentelemetry.io/otel/trace"
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

func WithTracer(tracer trace.Tracer) Option {
	return func(l *loaderConfig) {
		l.tracer = tracer
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
		thunkCache:   map[KeyT]func() (ValueT, error){},
	}
	return l
}

type loaderConfig struct {
	// how long to done before sending a batch
	wait time.Duration

	// this will limit the maximum number of keys to send in one batch, 0 = no limit
	maxBatch int

	tracer trace.Tracer
}

// Loader batches and caches requests
type Loader[KeyT comparable, ValueT any] struct {
	// this method provides the data for the loader
	fetch func(keys []KeyT) ([]ValueT, []error)

	*loaderConfig

	// INTERNAL

	// lazily created thunkCache
	thunkCache map[KeyT]func() (ValueT, error)

	// the current batch. keys will continue to be collected until timeout is hit,
	// then everything will be sent to the fetch method and out to the listeners
	batch *loaderBatch[KeyT, ValueT]

	// mutex to prevent races
	mu sync.Mutex
}

type loaderBatch[KeyT comparable, ValueT any] struct {
	keys          []KeyT
	results       []ValueT
	errors        []error
	fetchExecuted bool
	done          chan struct{}
	contexts      []context.Context
}

// Load a ValueT by key, batching and caching will be applied automatically
func (l *Loader[KeyT, ValueT]) Load(ctx context.Context, key KeyT) (ValueT, error) {
	return l.LoadThunk(loadContext, key)()
}

// LoadThunk returns a function that when called will block waiting for a ValueT.
// This method should be used if you want one goroutine to make requests to many
// different data loaders without blocking until the thunk is called.
func (l *Loader[KeyT, ValueT]) LoadThunk(ctx context.Context, key KeyT) func() (ValueT, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if it, ok := l.thunkCache[key]; ok {
		return it
	}

	loadContext, span := l.tracer.Start(ctx, "dataloadgen.load")
	defer span.End()

	l.startBatch()

	l.batch.contexts = append(l.batch.contexts, ctx)
	batch := l.batch
	pos := l.addKeyToBatch(batch, key)

	thunk := func() (ValueT, error) {
		_, span := l.tracer.Start(ctx, "dataloadgen.wait")
		<-batch.done
		span.End()

		var data ValueT

		// Return early if there's a single error and it's not nil
		if len(batch.errors) == 1 && batch.errors[0] != nil {
			return data, batch.errors[0]
		}

		// If the batch function returned the wrong number of responses, return an error to all callers
		if len(batch.results) != len(batch.keys) {
			return data, fmt.Errorf("bug in loader: %d values returned for %d keys", len(batch.results), len(batch.keys))
		}

		if pos < len(batch.results) {
			data = batch.results[pos]
		}

		var err error
		if batch.errors != nil {
			err = batch.errors[pos]
		}

		return data, err
	}
	l.thunkCache[key] = thunk
	return thunk
}

// LoadAll fetches many keys at once. It will be broken into appropriate sized
// sub batches depending on how the loader is configured
func (l *Loader[KeyT, ValueT]) LoadAll(ctx context.Context, keys []KeyT) ([]ValueT, []error) {
	thunks := make([]func() (ValueT, error), len(keys))

	for i, key := range keys {
		thunks[i] = l.LoadThunk(ctx, key)
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
func (l *Loader[KeyT, ValueT]) LoadAllThunk(ctx context.Context, keys []KeyT) func() ([]ValueT, []error) {
	thunks := make([]func() (ValueT, error), len(keys))
	for i, key := range keys {
		thunks[i] = l.LoadThunk(ctx, key)
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
	if _, found = l.thunkCache[key]; !found {
		l.thunkCache[key] = func() (ValueT, error) { return value, nil }
	}
	l.mu.Unlock()
	return !found
}

// Clear the value at key from the cache, if it exists
func (l *Loader[KeyT, ValueT]) Clear(key KeyT) {
	l.mu.Lock()
	delete(l.thunkCache, key)
	l.mu.Unlock()
}

func (l *Loader[KeyT, ValueT]) startBatch() {
	if l.batch == nil {
		batch := &loaderBatch[KeyT, ValueT]{done: make(chan struct{})}
		l.batch = batch
		go func(l *Loader[KeyT, ValueT]) {
			time.Sleep(l.wait)
			l.mu.Lock()

			// we must have hit a batch limit and are already finalizing this batch
			if batch.fetchExecuted {
				l.mu.Unlock()
				return
			}

			ctxs := l.batch.contexts
			l.batch = nil
			l.mu.Unlock()

			for _, ctx := range ctxs {
				_, span := l.tracer.Start(ctx, "dataloadgen.fetch.timelimit")
				defer span.End()
			}

			batch.results, batch.errors = l.fetch(batch.keys)
			close(batch.done)
		}(l)
	}
}

// addKeyToBatch will return the location of the key in the batch, if its not found
// it will add the key to the batch
func (l *Loader[KeyT, ValueT]) addKeyToBatch(b *loaderBatch[KeyT, ValueT], key KeyT) int {
	pos := len(b.keys)
	b.keys = append(b.keys, key)

	if l.maxBatch != 0 && pos >= l.maxBatch-1 {
		ctxs := l.batch.contexts
		b.fetchExecuted = true
		l.batch = nil
		go func(l *Loader[KeyT, ValueT], ctxs []context.Context) {
			for _, ctx := range ctxs {
				_, span := l.tracer.Start(ctx, "dataloadgen.fetch.keylimit")
				defer span.End()
			}
			b.results, b.errors = l.fetch(b.keys)
			close(b.done)
		}(l, ctxs)
	}

	return pos
}
