package benchmark_test

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/vikstrous/dataloadgen"
	"github.com/vikstrous/dataloadgen/benchmark"
	yckaodataloader "github.com/yckao/go-dataloader"
)

func newDataloader() *dataloader.Loader[int, benchmark.User] {
	return dataloader.NewBatchedLoader(func(ctx context.Context, keys []int) []*dataloader.Result[benchmark.User] {
		users := make([]*dataloader.Result[benchmark.User], len(keys))

		for i, key := range keys {
			if key%100 == 1 {
				users[i] = &dataloader.Result[benchmark.User]{Error: fmt.Errorf("user not found")}
			} else {
				users[i] = &dataloader.Result[benchmark.User]{Data: benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}}
			}
		}
		return users
	},
		dataloader.WithBatchCapacity[int, benchmark.User](100),
		dataloader.WithWait[int, benchmark.User](500*time.Nanosecond),
	)
}

func newDataloaden() *benchmark.UserLoader {
	return benchmark.NewUserLoader(benchmark.UserLoaderConfig{
		Wait:     500 * time.Nanosecond,
		MaxBatch: 100,
		Fetch: func(keys []int) ([]benchmark.User, []error) {
			users := make([]benchmark.User, len(keys))
			errors := make([]error, len(keys))

			for i, key := range keys {
				if key%100 == 1 {
					errors[i] = fmt.Errorf("user not found")
				} else {
					users[i] = benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}
				}
			}
			return users, errors
		},
	})
}

func newYckao() yckaodataloader.DataLoader[int, benchmark.User, int] {
	return yckaodataloader.New[int, benchmark.User, int](context.Background(), func(_ context.Context, keys []int) []yckaodataloader.Result[benchmark.User] {
		results := make([]yckaodataloader.Result[benchmark.User], len(keys))

		for i, key := range keys {
			if key%100 == 1 {
				results[i] = yckaodataloader.Result[benchmark.User]{Error: fmt.Errorf("user not found")}
			} else {
				results[i] = yckaodataloader.Result[benchmark.User]{Value: benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}}
			}
		}
		return results
	},
		yckaodataloader.WithMaxBatchSize[int, benchmark.User, int](100),
		yckaodataloader.WithBatchScheduleFn[int, benchmark.User, int](yckaodataloader.NewTimeWindowScheduler(500*time.Nanosecond)),
	)
}

func newVikstrous() *dataloadgen.Loader[int, benchmark.User] {
	return dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]benchmark.User, []error) {
		users := make([]benchmark.User, len(keys))
		errors := make([]error, len(keys))

		for i, key := range keys {
			if key%100 == 1 {
				errors[i] = fmt.Errorf("user not found")
			} else {
				users[i] = benchmark.User{ID: strconv.Itoa(key), Name: "user " + strconv.Itoa(key)}
			}
		}
		return users, errors
	},
		dataloadgen.WithBatchCapacity(100),
		dataloadgen.WithWait(500*time.Nanosecond),
	)
}

func BenchmarkAll(b *testing.B) {
	ctx := context.Background()

	b.Run("init", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				newDataloader()
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				newDataloaden()
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				newYckao()
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				newVikstrous()
			}
		})
	})

	b.Run("cached", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dataloaderDL := newDataloader()
				thunks := make([]func() (benchmark.User, error), 100)
				for i := 0; i < 100; i++ {
					thunks[i] = dataloaderDL.Load(ctx, 1)
				}
				for i := 0; i < 100; i++ {
					thunks[i]()
				}
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dataloadenDL := newDataloaden()
				thunks := make([]func() (benchmark.User, error), 100)
				for i := 0; i < 100; i++ {
					thunks[i] = dataloadenDL.LoadThunk(1)
				}

				for i := 0; i < 100; i++ {
					thunks[i]()
				}
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				yckaoDL := newYckao()
				thunks := make([]*yckaodataloader.Thunk[benchmark.User], 100)
				for i := 0; i < 100; i++ {
					thunks[i] = yckaoDL.Load(ctx, 1)
				}

				for i := 0; i < 100; i++ {
					thunks[i].Get(ctx)
				}
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				vikstrousDL := newVikstrous()
				thunks := make([]func() (benchmark.User, error), 100)
				for i := 0; i < 100; i++ {
					thunks[i] = vikstrousDL.LoadThunk(ctx, 1)
				}

				for i := 0; i < 100; i++ {
					thunks[i]()
				}
			}
		})
	})

	b.Run("unique keys", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dataloaderDL := newDataloader()
				thunks := make([]func() (benchmark.User, error), 100)
				for i := 0; i < 100; i++ {
					thunks[i] = dataloaderDL.Load(ctx, i)
				}
				for i := 0; i < 100; i++ {
					thunks[i]()
				}
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				dataloadenDL := newDataloaden()
				thunks := make([]func() (benchmark.User, error), 100)
				for i := 0; i < 100; i++ {
					thunks[i] = dataloadenDL.LoadThunk(i)
				}

				for i := 0; i < 100; i++ {
					thunks[i]()
				}
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				yckaoDL := newYckao()
				thunks := make([]*yckaodataloader.Thunk[benchmark.User], 100)
				for i := 0; i < 100; i++ {
					thunks[i] = yckaoDL.Load(ctx, i)
				}

				for i := 0; i < 100; i++ {
					thunks[i].Get(ctx)
				}
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				vikstrousDL := newVikstrous()
				thunks := make([]func() (benchmark.User, error), 100)
				for i := 0; i < 100; i++ {
					thunks[i] = vikstrousDL.LoadThunk(ctx, i)
				}

				for i := 0; i < 100; i++ {
					thunks[i]()
				}
			}
		})
	})
	b.Run("10 concurently", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			for n := 0; n < b.N*10; n++ {
				dataloaderDL := newDataloader()
				results := make([]benchmark.User, 10)
				var wg sync.WaitGroup
				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func(i int) {
						for j := 0; j < b.N; j++ {
							u, _ := dataloaderDL.Load(ctx, i)()
							results[i] = u
						}
						wg.Done()
					}(i)
				}
				wg.Wait()
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			for n := 0; n < b.N*10; n++ {
				dataloadenDL := newDataloaden()
				results := make([]benchmark.User, 10)
				var wg sync.WaitGroup
				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func(i int) {
						for j := 0; j < b.N; j++ {
							u, _ := dataloadenDL.Load(i)
							results[i] = u
						}
						wg.Done()
					}(i)
				}
				wg.Wait()
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			for n := 0; n < b.N*10; n++ {
				yckaoDL := newYckao()
				results := make([]benchmark.User, 10)
				var wg sync.WaitGroup
				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func(i int) {
						for j := 0; j < b.N; j++ {
							u, _ := yckaoDL.Load(ctx, i).Get(ctx)
							results[i] = u
						}
						wg.Done()
					}(i)
				}
				wg.Wait()
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			for n := 0; n < b.N*10; n++ {
				vikstrousDL := newVikstrous()
				results := make([]benchmark.User, 10)
				var wg sync.WaitGroup
				for i := 0; i < 10; i++ {
					wg.Add(1)
					go func(i int) {
						for j := 0; j < b.N; j++ {
							u, _ := vikstrousDL.Load(ctx, i)
							results[i] = u
						}
						wg.Done()
					}(i)
				}
				wg.Wait()
			}
		})
	})

	b.Run("all in one request", func(b *testing.B) {
		b.Run("dataloader", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < 10000; n++ {
				keys = append(keys, n)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dataloaderDL := newDataloader()
				dataloaderDL.LoadMany(ctx, keys)()
			}
		})
		b.Run("dataloaden", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < 10000; n++ {
				keys = append(keys, n)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				dataloadenDL := newDataloaden()
				dataloadenDL.LoadAll(keys)
			}
		})
		b.Run("yckao_dataloader", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < 10000; n++ {
				keys = append(keys, n)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				yckaoDL := newYckao()
				thunks := yckaoDL.LoadMany(ctx, keys)
				for _, t := range thunks {
					t.Get(ctx)
				}
			}
		})
		b.Run("dataloadgen", func(b *testing.B) {
			keys := []int{}
			for n := 0; n < 10000; n++ {
				keys = append(keys, n)
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				vikstrousDL := newVikstrous()
				vikstrousDL.LoadAll(ctx, keys)
			}
		})
	})
}
