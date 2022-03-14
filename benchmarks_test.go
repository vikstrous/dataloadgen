package dataloadgen_test

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/graph-gophers/dataloader"
	"github.com/vektah/dataloaden/example"
	"github.com/vikstrous/dataloadgen"
)

// Benchmarks copied from https://github.com/vektah/dataloaden

type benchmarkUser struct {
	Name string
	ID   string
}

func BenchmarkDataloader(b *testing.B) {
	ctx := context.Background()
	dl := dataloader.NewBatchedLoader(func(ctx context.Context, keys dataloader.Keys) []*dataloader.Result {
		users := make([]*dataloader.Result, len(keys))

		for i, key := range keys {
			if rand.Int()%100 == 1 {
				users[i] = &dataloader.Result{Error: fmt.Errorf("user not found")}
			} else if rand.Int()%100 == 1 {
				users[i] = &dataloader.Result{}
			} else {
				users[i] = &dataloader.Result{Data: &benchmarkUser{ID: key.String(), Name: "user " + key.String()}}
			}
		}
		return users
	},
		dataloader.WithBatchCapacity(100),
		dataloader.WithWait(500*time.Nanosecond),
	)

	b.Run("caches", func(b *testing.B) {
		thunks := make([]func() (interface{}, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.Load(ctx, dataloader.StringKey(strconv.Itoa(rand.Int()%300)))
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("random spread", func(b *testing.B) {
		thunks := make([]func() (interface{}, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.Load(ctx, dataloader.StringKey(strconv.Itoa(rand.Int())))
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("concurently", func(b *testing.B) {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				for j := 0; j < b.N; j++ {
					dl.Load(ctx, dataloader.StringKey(strconv.Itoa(rand.Int())))()
				}
				wg.Done()
			}()
		}
		wg.Wait()
	})
}

func BenchmarkDataloadgen(b *testing.B) {
	dl := dataloadgen.NewLoader(func(keys []string) ([]benchmarkUser, []error) {
		users := make([]benchmarkUser, len(keys))
		errors := make([]error, len(keys))

		for i, key := range keys {
			if rand.Int()%100 == 1 {
				errors[i] = fmt.Errorf("user not found")
			} else if rand.Int()%100 == 1 {
				users[i] = benchmarkUser{}
			} else {
				users[i] = benchmarkUser{ID: key, Name: "user " + key}
			}
		}
		return users, errors
	},
		dataloadgen.WithBatchCapacity[string, benchmarkUser](100),
		dataloadgen.WithWait[string, benchmarkUser](500*time.Nanosecond),
	)

	b.Run("caches", func(b *testing.B) {
		thunks := make([]func() (benchmarkUser, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(strconv.Itoa(rand.Int() % 300))
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("random spread", func(b *testing.B) {
		thunks := make([]func() (benchmarkUser, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(strconv.Itoa(rand.Int()))
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("concurently", func(b *testing.B) {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				for j := 0; j < b.N; j++ {
					dl.Load(strconv.Itoa(rand.Int()))
				}
				wg.Done()
			}()
		}
		wg.Wait()
	})
}
func BenchmarkDataloaden(b *testing.B) {
	dl := example.NewUserLoader(example.UserLoaderConfig{
		Wait:     500 * time.Nanosecond,
		MaxBatch: 100,
		Fetch: func(keys []string) ([]*example.User, []error) {
			users := make([]*example.User, len(keys))
			errors := make([]error, len(keys))

			for i, key := range keys {
				if rand.Int()%100 == 1 {
					errors[i] = fmt.Errorf("user not found")
				} else if rand.Int()%100 == 1 {
					users[i] = nil
				} else {
					users[i] = &example.User{ID: key, Name: "user " + key}
				}
			}
			return users, errors
		},
	})

	b.Run("caches", func(b *testing.B) {
		thunks := make([]func() (*example.User, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(strconv.Itoa(rand.Int() % 300))
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("random spread", func(b *testing.B) {
		thunks := make([]func() (*example.User, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(strconv.Itoa(rand.Int()))
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("concurently", func(b *testing.B) {
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				for j := 0; j < b.N; j++ {
					dl.Load(strconv.Itoa(rand.Int()))
				}
				wg.Done()
			}()
		}
		wg.Wait()
	})
}
