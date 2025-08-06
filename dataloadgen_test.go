package dataloadgen_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/vikstrous/dataloadgen"
)

func ExampleLoader() {
	ctx := context.Background()

	loader := dataloadgen.NewLoader(func(ctx context.Context, keys []string) (ret []int, errs []error) {
		for _, key := range keys {
			num, err := strconv.ParseInt(key, 10, 32)
			ret = append(ret, int(num))
			errs = append(errs, err)
		}
		return
	},
		dataloadgen.WithBatchCapacity(1),
		dataloadgen.WithWait(16*time.Millisecond),
	)
	one, err := loader.Load(ctx, "1")
	if err != nil {
		panic(err)
	}

	mappedLoader := dataloadgen.NewMappedLoader(func(ctx context.Context, keys []string) (ret map[string]int, err error) {
		ret = make(map[string]int, len(keys))
		errs := make(map[string]error, len(keys))
		for _, key := range keys {
			num, err := strconv.ParseInt(key, 10, 32)
			ret[key] = int(num)
			errs[key] = err
		}
		err = dataloadgen.MappedFetchError[string](errs)
		return
	},
		dataloadgen.WithBatchCapacity(1),
		dataloadgen.WithWait(16*time.Millisecond),
	)
	two, err := mappedLoader.Load(ctx, "2")
	if err != nil {
		panic(err)
	}
	fmt.Println(one, ",", two)
	// Output: 1 , 2
}

func TestCache(t *testing.T) {
	ctx := context.Background()
	var fetches [][]int
	var mu sync.Mutex
	dl := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]string, []error) {
		mu.Lock()
		fetches = append(fetches, keys)
		mu.Unlock()

		results := make([]string, len(keys))
		errs := make([]error, len(keys))

		for i, key := range keys {
			if key%2 == 0 {
				errs[i] = fmt.Errorf("not found")
			} else {
				results[i] = fmt.Sprint(key)
			}
		}
		return results, errs
	},
		dataloadgen.WithBatchCapacity(5),
		dataloadgen.WithWait(1*time.Millisecond),
	)

	for i := 0; i < 2; i++ {
		_, err := dl.Load(ctx, 0)
		if err == nil {
			t.Fatal("expected error")
		}
		if len(fetches) != 1 {
			t.Fatal("wrong number of fetches", fetches)
		}
		if len(fetches[0]) != 1 {
			t.Fatal("wrong number of keys in fetch request")
		}
	}
	for i := 0; i < 2; i++ {
		r, err := dl.Load(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(fetches) != 2 {
			t.Fatal("wrong number of fetches", fetches)
		}
		if len(fetches[1]) != 1 {
			t.Fatal("wrong number of keys in fetch request")
		}
		if r != "1" {
			t.Fatal("wrong data fetched", r)
		}
	}
}

// TestWithoutCache tests that the loader does not cache and reuse the results of a request.
func TestWithoutCache(t *testing.T) {
	ctx := context.Background()
	var fetches [][]int
	var mu sync.Mutex
	dl := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]string, []error) {
		mu.Lock()
		fetches = append(fetches, keys)
		mu.Unlock()

		results := make([]string, len(keys))
		errs := make([]error, len(keys))

		for i, key := range keys {
			if key%2 == 0 {
				errs[i] = fmt.Errorf("not found")
			} else {
				results[i] = fmt.Sprint(key)
			}
		}
		return results, errs
	},
		dataloadgen.WithBatchCapacity(5),
		dataloadgen.WithWait(1*time.Millisecond),
		dataloadgen.WithoutCache(),
	)

	for i := 0; i < 2; i++ {
		_, err := dl.Load(ctx, 0)
		if err == nil {
			t.Fatal("expected error")
		}
		nbExpectedFetches := i + 1
		if len(fetches) != nbExpectedFetches {
			t.Fatalf("wrong number of fetches, got %d expected %d", fetches, nbExpectedFetches)
		}
		if len(fetches[i]) != 1 {
			t.Fatal("wrong number of keys in fetch request")
		}
	}

	nbFetchesBefore := len(fetches)
	for i := 0; i < 2; i++ {
		r, err := dl.Load(ctx, 1)
		if err != nil {
			t.Fatalf("unexpected error %v", err)
		}
		nbExpectedFetches := nbFetchesBefore + i + 1
		if len(fetches) != nbExpectedFetches {
			t.Fatalf("wrong number of fetches, got %d expected %d", fetches, nbExpectedFetches)
		}
		j := nbFetchesBefore + i
		if len(fetches[j]) != 1 {
			t.Fatal("wrong number of keys in fetch request")
		}
		if r != "1" {
			t.Fatal("wrong data fetched", r)
		}
	}
}

// TestWithoutCachePrimed validates that primed cache values are not used when the cache is disabled.
func TestWithoutCachePrimed(t *testing.T) {
	ctx := context.Background()
	var fetches [][]int
	var mu sync.Mutex
	dl := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]string, []error) {
		mu.Lock()
		fetches = append(fetches, keys)
		mu.Unlock()

		results := make([]string, len(keys))
		errs := make([]error, len(keys))

		for i, key := range keys {
			if key%2 == 0 {
				errs[i] = fmt.Errorf("not found")
			} else {
				results[i] = fmt.Sprint(key)
			}
		}
		return results, errs
	},
		dataloadgen.WithBatchCapacity(5),
		dataloadgen.WithWait(1*time.Millisecond),
		dataloadgen.WithoutCache(),
	)

	if ok := dl.Prime(1, "primed-1"); ok {
		t.Fatal("expected Prime to return false when using WithoutCache")
	}

	res_0, err_0 := dl.Load(ctx, 0)
	if err_0 == nil {
		t.Fatal("expected error for key 0")
	}
	if res_0 != "" {
		t.Fatal("expected '' for key 0, got no error")
	}
	if len(fetches) != 1 {
		t.Fatalf("expected 1 fetch, got %d", len(fetches))
	}

	res_1, err_1 := dl.Load(ctx, 1)
	if err_1 != nil {
		t.Fatalf("unexpected error for key 1: %v", err_0)
	}
	if res_1 != "1" {
		t.Fatalf("expected '1' for key 1, got %s", res_1)
	}
	if len(fetches) != 2 {
		t.Fatalf("expected 2 fetches, got %d", len(fetches))
	}

	res_2, err_2 := dl.Load(ctx, 2)
	if err_2 == nil {
		t.Fatalf("expected error for key 2, got no error")
	}
	if res_2 != "" {
		t.Fatalf("expected '' for key 2, got %s", res_2)
	}
	if len(fetches) != 3 {
		t.Fatalf("expected 3 fetches, got %d", len(fetches))
	}

	res_1_again, err_1_again := dl.Load(ctx, 1)
	if err_1_again != nil {
		t.Fatalf("unexpected error for key 1: %v", err_0)
	}
	if res_1_again != "1" {
		t.Fatalf("expected '1' for key 1, got %s", res_1_again)
	}

	wantFetches := [][]int{
		{0},
		{1},
		{2},
		{1},
	}
	if !reflect.DeepEqual(wantFetches, fetches) {
		t.Fatalf("expected fetches to be %v, got %v", wantFetches, fetches)
	}
}

func TestErrors(t *testing.T) {
	ctx := context.Background()
	dl := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]string, []error) {
		return []string{"1", "2", "3"}, []error{fmt.Errorf("error 1"), fmt.Errorf("error 2")}
	},
		dataloadgen.WithBatchCapacity(3),
	)
	_, err := dl.LoadAll(ctx, []int{1, 2, 3})
	var errs dataloadgen.ErrorSlice
	errors.As(err, &errs)
	if len(errs) != 3 {
		t.Fatalf("wrong number of errors: %d", len(errs))
	}
	if errs[0].Error() != "error 1" {
		t.Fatalf("wrong error: %s", errs[0].Error())
	}
	if errs[1].Error() != "error 2" {
		t.Fatalf("wrong error: %s", errs[1].Error())
	}
	if errs[2].Error() != "bug in fetch function: 2 errors returned for 3 keys; last error: error 2" {
		t.Fatalf("wrong error: %s", errs[2].Error())
	}
}

func TestErrorSliceUnwrap(t *testing.T) {
	ctx := context.Background()
	dl := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]string, []error) {
		return []string{"1", "2", "3"}, []error{fmt.Errorf("error 1"), context.Canceled}
	},
		dataloadgen.WithBatchCapacity(3),
	)
	_, err := dl.LoadAll(ctx, []int{1, 2, 3})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error does not unwrap: %s", err)
	}
}

func TestPanic(t *testing.T) {
	ctx := context.Background()
	dl := dataloadgen.NewLoader(func(_ context.Context, keys []int) ([]string, []error) {
		panic("fetch panic")
	},
		dataloadgen.WithBatchCapacity(1),
	)
	_, err := dl.Load(ctx, 1)
	if err != nil && err.Error() != "panic during fetch: fetch panic" {
		t.Fatalf("wrong error: %s", err.Error())
	}
}

func TestMappedLoader(t *testing.T) {
	ctx := context.Background()
	dl := dataloadgen.NewMappedLoader(func(_ context.Context, keys []string) (res map[string]*string, err error) {
		one := "1"
		res = map[string]*string{"1": &one}
		err = dataloadgen.MappedFetchError[string](map[string]error{"3": errors.New("not found error")})
		return
	})

	thunkOne := dl.LoadThunk(ctx, "1")
	thunkTwo := dl.LoadThunk(ctx, "2")
	thunkThree := dl.LoadThunk(ctx, "3")

	one, _ := thunkOne()
	two, errTwo := thunkTwo()
	_, errThree := thunkThree()

	if *one != "1" {
		t.Fatal("wrong value returned for '1':", *one)
	}
	if two != nil || !errors.Is(errTwo, dataloadgen.ErrNotFound) {
		t.Fatalf("wrong value/err returned for '2'. Value: %v Err: %v", two, errTwo)
	}
	if errThree == nil || errThree.Error() != "not found error" {
		t.Fatal("wrong error:", errThree)
	}
}

func TestMappedLoaderSingleError(t *testing.T) {
	ctx := context.Background()
	dl := dataloadgen.NewMappedLoader(func(_ context.Context, keys []string) (res map[string]*string, err error) {
		err = errors.New("something went wrong")
		return
	})

	thunkOne := dl.LoadThunk(ctx, "1")
	thunkTwo := dl.LoadThunk(ctx, "2")
	thunkThree := dl.LoadThunk(ctx, "3")

	_, errOne := thunkOne()
	_, errTwo := thunkTwo()
	_, errThree := thunkThree()

	if errOne != nil && errTwo != nil && errThree != nil {
		if errors.Is(errTwo, errOne) && errors.Is(errThree, errTwo) {
			if errOne.Error() != "something went wrong" {
				t.Fatalf("Unexpected error message: %s", errOne.Error())
			}
		} else {
			t.Fatalf("All errors should be equal, instead got: %s, %s, %s", errOne, errTwo, errThree)
		}
	} else {
		t.Fatalf("All errors should be non-nil, instead got: %s, %s, %s", errOne, errTwo, errThree)
	}
}

func TestMappedLoaderNotFoundError(t *testing.T) {
	ctx := context.Background()
	dl := dataloadgen.NewMappedLoader(func(_ context.Context, keys []string) (map[string]*string, error) {
		return nil, nil
	})
	_, err := dl.Load(ctx, "1")
	if !errors.Is(err, dataloadgen.ErrNotFound) {
		t.Fatalf("Wrong error returned: %T", err)
	}

	dl2 := dataloadgen.NewMappedLoader(func(_ context.Context, keys []string) (map[string]*string, error) {
		return map[string]*string{}, nil
	})
	_, err = dl2.Load(ctx, "1")
	if !errors.Is(err, dataloadgen.ErrNotFound) {
		t.Fatalf("Wrong error returned: %T", err)
	}
}
