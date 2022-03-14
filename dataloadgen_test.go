package dataloadgen_test

import (
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

func TestEdgeCases(t *testing.T) {
	var fetches [][]int
	var mu sync.Mutex
	dl := dataloadgen.NewLoader(func(keys []int) ([]string, []error) {
		mu.Lock()
		fetches = append(fetches, keys)
		mu.Unlock()

		results := make([]string, len(keys))
		errors := make([]error, len(keys))

		for i, key := range keys {
			if key%2 == 0 {
				errors[i] = fmt.Errorf("not found")
			} else {
				results[i] = fmt.Sprint(key)
			}
		}
		return results, errors
	},
		dataloadgen.WithBatchCapacity[int, string](5),
		dataloadgen.WithWait[int, string](1*time.Millisecond),
	)

	t.Run("load function called only once when cached", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			_, err := dl.Load(0)
			require.Error(t, err)
			require.Len(t, fetches, 1)
			require.Len(t, fetches[0], 1)
		}
		for i := 0; i < 2; i++ {
			r, err := dl.Load(1)
			require.NoError(t, err)
			require.Len(t, fetches, 2)
			require.Len(t, fetches[1], 1)
			require.Equal(t, "1", r)
		}
	})
}
