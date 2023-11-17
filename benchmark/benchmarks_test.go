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
	"github.com/vikstrous/dataloadgen/benchmark"
	yckaodataloader "github.com/yckao/go-dataloader"
)

func BenchmarkAll(b *testing.B) {
	ctx := context.Background()
	dataloaderDL := dataloader.NewBatchedLoader(func(ctx context.Context, keys []int) []*dataloader.Result[benchmark.User] {
		users := make([]*dataloader.Result[benchmark.User], len(keys))

		for i, key := range keys {
			if key%100 == 1 {
				users[i] = &dataloader.Result[benchmark.User]{Error: fmt.Errorf("user not found")}
			} else if key%100 == 1 {
				users[i] = &dataloader.Result[benchmark.User]{}
			} else {
				users[i] = &dataloader.Result[benchmark.User]{Data: benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}}
			}
		}
		return users
	},
		dataloader.WithBatchCapacity[int, benchmark.User](100),
		dataloader.WithWait[int, benchmark.User](500*time.Nanosecond),
	)
	dataloadenDL := benchmark.NewUserLoader(benchmark.UserLoaderConfig{
		Wait:     500 * time.Nanosecond,
		MaxBatch: 100,
		Fetch: func(keys []int) ([]benchmark.User, []error) {
			users := make([]benchmark.User, len(keys))
			errors := make([]error, len(keys))

			for i, key := range keys {
				if key%100 == 1 {
					errors[i] = fmt.Errorf("user not found")
				} else if key%100 == 1 {
					users[i] = benchmark.User{}
				} else {
					users[i] = benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}
				}
			}
			return users, errors
		},
	})
	yckaoDL := yckaodataloader.New[int, benchmark.User, int](ctx, func(_ context.Context, keys []int) []yckaodataloader.Result[benchmark.User] {
		results := make([]yckaodataloader.Result[benchmark.User], len(keys))

		for i, key := range keys {
			if key%100 == 1 {
				results[i] = yckaodataloader.Result[benchmark.User]{Error: fmt.Errorf("user not found")}
			} else if key%100 == 1 {
				results[i] = yckaodataloader.Result[benchmark.User]{Value: benchmark.User{}}
			} else {
				results[i] = yckaodataloader.Result[benchmark.User]{Value: benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}}
			}
		}
		return results
	},
		yckaodataloader.WithMaxBatchSize[int, benchmark.User, int](100),
		yckaodataloader.WithBatchScheduleFn[int, benchmark.User, int](yckaodataloader.NewTimeWindowScheduler(500*time.Nanosecond)),
	)
	vikstrousDL := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]benchmark.User, []error) {
		users := make([]benchmark.User, len(keys))
		errors := make([]error, len(keys))

		for i, key := range keys {
			if key%100 == 1 {
				errors[i] = fmt.Errorf("user not found")
			} else if key%100 == 1 {
				users[i] = benchmark.User{}
			} else {
				users[i] = benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}
			}
		}
		return users, errors
	},
		dataloadgen.WithBatchCapacity(100),
		dataloadgen.WithWait(500*time.Nanosecond),
	)

	b.Run("caches", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int()%300)
			}
			b.ResetTimer()
			thunks := make([]func() (benchmark.User, error), b.N)
			for i := 0; i < b.N; i++ {
				thunks[i] = dataloaderDL.Load(ctx, queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i]()
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int()%300)
			}
			b.ResetTimer()
			thunks := make([]func() (benchmark.User, error), b.N)
			for i := 0; i < b.N; i++ {
				thunks[i] = dataloadenDL.LoadThunk(queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i]()
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int()%300)
			}
			b.ResetTimer()
			thunks := make([]*yckaodataloader.Thunk[benchmark.User], b.N)
			for i := 0; i < b.N; i++ {
				thunks[i] = yckaoDL.Load(ctx, queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i].Get(ctx)
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int()%300)
			}
			b.ResetTimer()
			thunks := make([]func() (benchmark.User, error), b.N)
			for i := 0; i < b.N; i++ {
				thunks[i] = vikstrousDL.LoadThunk(ctx, queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i]()
			}
		})
	})

	b.Run("random spread", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int())
			}
			thunks := make([]func() (benchmark.User, error), b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				thunks[i] = dataloaderDL.Load(ctx, queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i]()
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int())
			}
			thunks := make([]func() (benchmark.User, error), b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				thunks[i] = dataloadenDL.LoadThunk(queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i]()
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int())
			}
			thunks := make([]*yckaodataloader.Thunk[benchmark.User], b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				thunks[i] = yckaoDL.Load(ctx, queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i].Get(ctx)
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N; n++ {
				queries = append(queries, rand.Int())
			}
			thunks := make([]func() (benchmark.User, error), b.N)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				thunks[i] = vikstrousDL.LoadThunk(ctx, queries[i])
			}

			for i := 0; i < b.N; i++ {
				thunks[i]()
			}
		})
	})
	b.Run("10 concurently", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N*10; n++ {
				queries = append(queries, rand.Int())
			}
			results := make([]benchmark.User, b.N*10)
			b.ResetTimer()
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(i int) {
					for j := 0; j < b.N; j++ {
						u, _ := dataloaderDL.Load(ctx, queries[j+i*b.N])()
						results[j+i*b.N] = u
					}
					wg.Done()
				}(i)
			}
			wg.Wait()
		})
		b.Run("dataloaden", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < b.N*10; n++ {
				queries = append(queries, rand.Int())
			}
			results := make([]benchmark.User, b.N*10)
			b.ResetTimer()
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(i int) {
					for j := 0; j < b.N; j++ {
						u, _ := dataloadenDL.Load(queries[j+i*b.N])
						results[j+i*b.N] = u
					}
					wg.Done()
				}(i)
			}
			wg.Wait()
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < 10*b.N; n++ {
				queries = append(queries, rand.Int())
			}
			results := make([]benchmark.User, b.N*10)
			b.ResetTimer()
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(i int) {
					for j := 0; j < b.N; j++ {
						u, _ := yckaoDL.Load(ctx, queries[j+i*b.N]).Get(ctx)
						results[j+i*b.N] = u
					}
					wg.Done()
				}(i)
			}
			wg.Wait()
		})
		b.Run("dataloadgen", func(b *testing.B) {
			queries := []int{}
			for n := 0; n < 10*b.N; n++ {
				queries = append(queries, rand.Int())
			}
			results := make([]benchmark.User, b.N*10)
			b.ResetTimer()
			var wg sync.WaitGroup
			for i := 0; i < 10; i++ {
				wg.Add(1)
				go func(i int) {
					for j := 0; j < b.N; j++ {
						u, _ := vikstrousDL.Load(ctx, queries[j+i*b.N])
						results[j+i*b.N] = u
					}
					wg.Done()
				}(i)
			}
			wg.Wait()
		})
	})

	b.Run("all in one request", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < b.N; n++ {
				keys = append(keys, rand.Int())
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dataloaderDL.LoadMany(ctx, keys)()
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < b.N; n++ {
				keys = append(keys, rand.Int())
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dataloadenDL.LoadAll(keys)
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < b.N; n++ {
				keys = append(keys, rand.Int())
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				thunks := yckaoDL.LoadMany(ctx, keys)
				for _, t := range thunks {
					t.Get(ctx)
				}
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < b.N; n++ {
				keys = append(keys, rand.Int())
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				vikstrousDL.LoadAll(ctx, keys)
			}
		})
	})
}
