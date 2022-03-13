package dataloadgen_test

import (
	"fmt"
	"math/rand"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/vikstrous/dataloadgen"
)

func ExampleLoader() {
	loader := dataloadgen.NewLoader(func(keys []string) (ret []int, errs []error) {
		for _, key := range keys {
			num, err := strconv.ParseInt(key, 10, 32)
			ret = append(ret, int(num))
			errs = append(errs, err)
		}
		return
	},
		dataloadgen.WithBatchCapacity[string, int](1),
		dataloadgen.WithWait[string, int](16*time.Millisecond),
	)
	one, err := loader.Load("1")
	if err != nil {
		panic(err)
	}
	fmt.Println(one)
	// Output: 1
}

type benchmarkUser struct {
	ID   string
	Name string
}

func BenchmarkLoaderFromDataloaden(b *testing.B) {
	dl := dataloadgen.NewLoader(func(keys []string) ([]*benchmarkUser, []error) {
		users := make([]*benchmarkUser, len(keys))
		errors := make([]error, len(keys))

		for i, key := range keys {
			if rand.Int()%100 == 1 {
				errors[i] = fmt.Errorf("user not found")
			} else if rand.Int()%100 == 1 {
				users[i] = nil
			} else {
				users[i] = &benchmarkUser{ID: key, Name: "user " + key}
			}
		}
		return users, errors
	},
		dataloadgen.WithBatchCapacity[string, *benchmarkUser](100),
		dataloadgen.WithWait[string, *benchmarkUser](500*time.Nanosecond),
	)

	b.Run("caches", func(b *testing.B) {
		thunks := make([]func() (*benchmarkUser, error), b.N)
		for i := 0; i < b.N; i++ {
			thunks[i] = dl.LoadThunk(strconv.Itoa(rand.Int() % 300))
		}

		for i := 0; i < b.N; i++ {
			thunks[i]()
		}
	})

	b.Run("random spread", func(b *testing.B) {
		thunks := make([]func() (*benchmarkUser, error), b.N)
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
