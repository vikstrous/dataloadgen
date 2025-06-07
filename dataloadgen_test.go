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
