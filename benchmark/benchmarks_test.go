package benchmark_test

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/vikstrous/dataloadgen"
)

// Benchmarks copied from https://github.com/vektah/dataloaden

type benchmarkUser struct {
	Name string
	ID   string
}

func BenchmarkDataloader(b *testing.B) {
	ctx := context.Background()
	dl := dataloader.NewBatchedLoader(func(ctx context.Context, keys []int) []*dataloader.Result[benchmarkUser] {
		users := make([]*dataloader.Result[benchmarkUser], len(keys))

		for i, key := range keys {
			if rand.Int()%100 == 1 {
				users[i] = &dataloader.Result[benchmarkUser]{Error: fmt.Errorf("user not found")}
			} else if rand.Int()%100 == 1 {
				users[i] = &dataloader.Result[benchmarkUser]{}
			} else {
				users[i] = &dataloader.Result[benchmarkUser]{Data: benchmarkUser{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}}
			}
		}
		return users
	},
		dataloader.WithBatchCapacity[int, benchmarkUser](100),
		dataloader.WithWait[int, benchmarkUser](500*time.Nanosecond),
	)

	b.Run("caches", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N; n++ {
			queries = append(queries, rand.Int()%300)
		}
		b.ResetTimer()
		thunks := make([]func() (benchmarkUser, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.Load(ctx, queries[i])
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("random spread", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N; n++ {
			queries = append(queries, rand.Int())
		}
		b.ResetTimer()
		thunks := make([]func() (benchmarkUser, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.Load(ctx, queries[i])
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("concurently", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N*10; n++ {
			queries = append(queries, rand.Int())
		}
		b.ResetTimer()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				for j := 0; j < b.N; j++ {
					dl.Load(ctx, queries[i*b.N+j])()
				}
				wg.Done()
			}(i)
		}
		wg.Wait()
	})

	b.Run("all in one request", func(b *testing.B) {
		keys := []int{}
		for n := 0; n < b.N; n++ {
			keys = append(keys, rand.Int())
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dl.LoadMany(ctx, keys)()
		}
	})
}

// IntKey implements the Key interface for an int
type IntKey int

// String is an identity method. Used to implement String interface
func (k IntKey) String() string { return strconv.Itoa(int(k)) }

// String is an identity method. Used to implement Key Raw
func (k IntKey) Raw() interface{} { return k }

func BenchmarkDataloadgen(b *testing.B) {
	ctx := context.Background()
	dl := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]benchmarkUser, []error) {
		users := make([]benchmarkUser, len(keys))
		errors := make([]error, len(keys))

		for i, key := range keys {
			if key%100 == 1 {
				errors[i] = fmt.Errorf("user not found")
			} else if key%100 == 1 {
				users[i] = benchmarkUser{}
			} else {
				users[i] = benchmarkUser{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}
			}
		}
		return users, errors
	},
		dataloadgen.WithBatchCapacity(100),
		dataloadgen.WithWait(500*time.Nanosecond),
	)

	b.Run("caches", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N; n++ {
			queries = append(queries, rand.Int()%300)
		}
		b.ResetTimer()
		thunks := make([]func() (benchmarkUser, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(ctx, queries[i])
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("random spread", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N; n++ {
			queries = append(queries, rand.Int())
		}
		b.ResetTimer()
		thunks := make([]func() (benchmarkUser, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(ctx, queries[i])
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("concurently", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < 10*b.N; n++ {
			queries = append(queries, rand.Int())
		}
		b.ResetTimer()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				for j := 0; j < b.N; j++ {
					dl.Load(ctx, queries[j+i*b.N])
				}
				wg.Done()
			}(i)
		}
		wg.Wait()
	})

	b.Run("all in one request", func(b *testing.B) {
		keys := []int{}
		for n := 0; n < b.N; n++ {
			keys = append(keys, rand.Int())
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dl.LoadAll(ctx, keys)
		}
	})
}

func BenchmarkDataloaden(b *testing.B) {
	dl := NewUserLoader(UserLoaderConfig{
		Wait:     500 * time.Nanosecond,
		MaxBatch: 100,
		Fetch: func(keys []int) ([]*User, []error) {
			users := make([]*User, len(keys))
			errors := make([]error, len(keys))

			for i, key := range keys {
				if rand.Int()%100 == 1 {
					errors[i] = fmt.Errorf("user not found")
				} else if rand.Int()%100 == 1 {
					users[i] = nil
				} else {
					users[i] = &User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}
				}
			}
			return users, errors
		},
	})

	b.Run("caches", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N; n++ {
			queries = append(queries, rand.Int()%300)
		}
		b.ResetTimer()
		thunks := make([]func() (*User, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(queries[i])
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("random spread", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N; n++ {
			queries = append(queries, rand.Int())
		}
		b.ResetTimer()
		thunks := make([]func() (*User, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(queries[i])
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("concurently", func(b *testing.B) {
		queries := []int{}
		for n := 0; n < b.N*10; n++ {
			queries = append(queries, rand.Int())
		}
		b.ResetTimer()
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(i int) {
				for j := 0; j < b.N; j++ {
					dl.Load(queries[i*b.N+j])
				}
				wg.Done()
			}(i)
		}
		wg.Wait()
	})

	b.Run("all in one request", func(b *testing.B) {
		keys := []int{}
		for n := 0; n < b.N; n++ {
			keys = append(keys, rand.Int())
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			dl.LoadAll(keys)
		}
	})
}
