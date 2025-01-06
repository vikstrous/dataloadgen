package dataloadgen_test

import (
	"context"
	"errors"
	"fmt"
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

	mappedLoader := dataloadgen.NewLoader(func(ctx context.Context, keys []string) (ret []int, errs []error) {
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
	dl := dataloadgen.NewMappedLoader(func(_ context.Context, keys []string) (map[string]*string, map[string]error) {
		one := "1"
		results := map[string]*string{"1": &one}
		errs := map[string]error{"3": errors.New("not found error")}
		return results, errs
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
	if two != nil || errTwo != nil {
		t.Fatalf("wrong value/err returned for '2'. Value: %v Err: %v", two, errTwo)
	}
	if errThree == nil || errThree.Error() != "not found error" {
		t.Fatal("wrong error:", errThree)
	}
}
