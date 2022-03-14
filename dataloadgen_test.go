package dataloadgen_test

import (
	"errors"
	"fmt"
	"math/rand"
	"reflect"
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

// copied from https://github.com/vektah/dataloaden
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

// copied and adapted from github.com/graph-gophers/dataloader
func BenchmarkLoaderFromDataloader(b *testing.B) {
	var a = &Avg{}
	dl := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		a.Add(len(keys))
		results = make([]string, 0, len(keys))
		for _, key := range keys {
			results = append(results, key)
		}
		return results, nil
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dl.LoadThunk(strconv.Itoa(i))
	}
	b.Logf("avg: %f", a.Avg())
}

type Avg struct {
	total  float64
	length float64
	lock   sync.RWMutex
}

func (a *Avg) Add(v int) {
	a.lock.Lock()
	a.total += float64(v)
	a.length++
	a.lock.Unlock()
}

func (a *Avg) Avg() float64 {
	a.lock.RLock()
	defer a.lock.RUnlock()
	if a.total == 0 {
		return 0
	} else if a.length == 0 {
		return 0
	}
	return a.total / a.length
}

func TestLoader(t *testing.T) {
	t.Run("test Load method", func(t *testing.T) {
		t.Parallel()
		identityLoader, _ := IDLoader(0)
		value, err := identityLoader.Load("1")
		if err != nil {
			t.Error(err.Error())
		}
		if value != "1" {
			t.Error("load didn't return the right value")
		}
	})

	t.Run("test thunk does not contain race conditions", func(t *testing.T) {
		t.Parallel()
		identityLoader, _ := IDLoader(0)
		future := identityLoader.LoadThunk("1")
		go future()
		go future()
	})

	t.Run("test Load Method Panic Safety", func(t *testing.T) {
		t.Skip("not supported yet")
		t.Parallel()
		defer func() {
			r := recover()
			if r != nil {
				t.Error("Panic Loader's panic should have been handled'")
			}
		}()
		panicLoader, _ := PanicLoader(0)
		_, err := panicLoader.Load("1")
		if err == nil || err.Error() != "Panic received in batch function: Programming error" {
			t.Error("Panic was not propagated as an error.")
		}
	})

	t.Run("test Load Method Panic Safety in multiple keys", func(t *testing.T) {
		t.Skip("not supported yet")
		t.Parallel()
		defer func() {
			r := recover()
			if r != nil {
				t.Error("Panic Loader's panic should have been handled'")
			}
		}()
		panicLoader, _ := PanicLoader(0)
		futures := []func() (string, error){}
		for i := 0; i < 3; i++ {
			futures = append(futures, panicLoader.LoadThunk(strconv.Itoa(i)))
		}
		for _, f := range futures {
			_, err := f()
			if err == nil || err.Error() != "Panic received in batch function: Programming error" {
				t.Error("Panic was not propagated as an error.")
			}
		}
	})

	t.Run("test LoadAll returns errors", func(t *testing.T) {
		t.Parallel()
		errorLoader, _ := ErrorLoader(0)
		_, err := errorLoader.LoadAll([]string{"1", "2", "3"})
		if len(err) != 3 {
			t.Error("LoadAll didn't return right number of errors")
		}
	})

	t.Run("test LoadAll returns len(errors) == len(keys)", func(t *testing.T) {
		t.Parallel()
		loader, _ := OneErrorLoader(3)
		_, errs := loader.LoadAll([]string{"1", "2", "3"})
		if len(errs) != 3 {
			t.Errorf("LoadAll didn't return right number of errors (should match size of input)")
		}

		var errCount int = 0
		var nilCount int = 0
		for _, err := range errs {
			if err == nil {
				nilCount++
			} else {
				errCount++
			}
		}
		if errCount != 1 {
			t.Error("Expected an error on only one of the items loaded")
		}

		if nilCount != 2 {
			t.Error("Expected second and third errors to be nil")
		}
	})

	t.Run("test LoadAll returns nil []error when no errors occurred", func(t *testing.T) {
		t.Parallel()
		loader, _ := IDLoader(0)
		_, errs := loader.LoadAll([]string{"1", "2", "3"})
		if errs != nil {
			t.Errorf("Expected LoadAll() to return nil error slice when no errors occurred")
		}
	})

	t.Run("test thunkmany does not contain race conditions", func(t *testing.T) {
		t.Parallel()
		identityLoader, _ := IDLoader(0)
		future := identityLoader.LoadAllThunk([]string{"1", "2", "3"})
		go future()
		go future()
	})

	t.Run("test Load Many Method Panic Safety", func(t *testing.T) {
		t.Skip("not supported yet")
		t.Parallel()
		defer func() {
			r := recover()
			if r != nil {
				t.Error("Panic Loader's panic should have been handled'")
			}
		}()
		panicLoader, _ := PanicLoader(0)
		_, errs := panicLoader.LoadAll([]string{"1"})
		if len(errs) < 1 || errs[0].Error() != "Panic received in batch function: Programming error" {
			t.Error("Panic was not propagated as an error.")
		}
	})

	t.Run("test LoadAll method", func(t *testing.T) {
		t.Parallel()
		identityLoader, _ := IDLoader(0)
		results, _ := identityLoader.LoadAll([]string{"1", "2", "3"})
		if results[0] != "1" || results[1] != "2" || results[2] != "3" {
			t.Error("LoadAll didn't return the right value")
		}
	})

	t.Run("batches many requests", func(t *testing.T) {
		t.Parallel()
		identityLoader, loadCalls := IDLoader(0)
		future1 := identityLoader.LoadThunk("1")
		future2 := identityLoader.LoadThunk("2")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future2()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1", "2"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not call batchFn in right order. Expected %#v, got %#v", expected, calls)
		}
	})

	t.Run("number of results matches number of keys", func(t *testing.T) {
		t.Parallel()
		faultyLoader, _ := FaultyLoader()

		n := 10
		reqs := []func() (string, error){}
		keys := []string{}
		for i := 0; i < n; i++ {
			key := strconv.Itoa(i)
			reqs = append(reqs, faultyLoader.LoadThunk(key))
			keys = append(keys, key)
		}

		for _, future := range reqs {
			_, err := future()
			if err == nil {
				t.Error("if number of results doesn't match keys, all keys should contain error")
			}
		}
	})

	t.Run("responds to max batch size", func(t *testing.T) {
		t.Parallel()
		identityLoader, loadCalls := IDLoader(2)
		future1 := identityLoader.LoadThunk("1")
		future2 := identityLoader.LoadThunk("2")
		future3 := identityLoader.LoadThunk("3")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future2()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future3()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner1 := []string{"1", "2"}
		inner2 := []string{"3"}
		expected := [][]string{inner1, inner2}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not respect max batch size. Expected %#v, got %#v", expected, calls)
		}
	})

	t.Run("caches repeated requests", func(t *testing.T) {
		t.Parallel()
		identityLoader, loadCalls := IDLoader(0)
		future1 := identityLoader.LoadThunk("1")
		future2 := identityLoader.LoadThunk("1")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future2()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not respect max batch size. Expected %#v, got %#v", expected, calls)
		}
	})

	t.Run("allows primed cache", func(t *testing.T) {
		t.Parallel()
		identityLoader, loadCalls := IDLoader(0)
		identityLoader.Prime("A", "Cached")
		future1 := identityLoader.LoadThunk("1")
		future2 := identityLoader.LoadThunk("A")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		value, err := future2()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not respect max batch size. Expected %#v, got %#v", expected, calls)
		}

		if value != "Cached" {
			t.Errorf("did not use primed cache value. Expected '%#v', got '%#v'", "Cached", value)
		}
	})

	t.Run("allows clear value in cache", func(t *testing.T) {
		t.Parallel()
		identityLoader, loadCalls := IDLoader(0)
		identityLoader.Prime("A", "Cached")
		identityLoader.Prime("B", "B")
		future1 := identityLoader.LoadThunk("1")
		identityLoader.Clear("A")
		future2 := identityLoader.LoadThunk("A")
		future3 := identityLoader.LoadThunk("B")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		value, err := future2()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future3()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1", "A"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not respect max batch size. Expected %#v, got %#v", expected, calls)
		}

		if value != "A" {
			t.Errorf("did not use primed cache value. Expected '%#v', got '%#v'", "Cached", value)
		}
	})

	t.Run("clears cache on batch with WithClearCacheOnBatch", func(t *testing.T) {
		t.Skip("not supported yet")
		t.Parallel()
		batchOnlyLoader, loadCalls := BatchOnlyLoader(0)
		future1 := batchOnlyLoader.LoadThunk("1")
		future2 := batchOnlyLoader.LoadThunk("1")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future2()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not batch queries. Expected %#v, got %#v", expected, calls)
		}

		//if _, found := batchOnlyLoader.cache.Get("1"); found {
		//	t.Errorf("did not clear cache after batch. Expected %#v, got %#v", false, found)
		//}
	})

	t.Run("allows clearAll values in cache", func(t *testing.T) {
		t.Skip("not supported yet")
		t.Parallel()
		identityLoader, loadCalls := IDLoader(0)
		identityLoader.Prime("A", "Cached")
		identityLoader.Prime("B", "B")

		// identityLoader.ClearAll()

		future1 := identityLoader.LoadThunk("1")
		future2 := identityLoader.LoadThunk("A")
		future3 := identityLoader.LoadThunk("B")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future2()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future3()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1", "A", "B"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not respect max batch size. Expected %#v, got %#v", expected, calls)
		}
	})

	t.Run("all methods on NoCache are Noops", func(t *testing.T) {
		t.Skip("not supported yet")
		t.Parallel()
		identityLoader, loadCalls := NoCacheLoader(0)
		identityLoader.Prime("A", "Cached")
		identityLoader.Prime("B", "B")

		// identityLoader.ClearAll()

		identityLoader.Clear("1")
		future1 := identityLoader.LoadThunk("1")
		future2 := identityLoader.LoadThunk("A")
		future3 := identityLoader.LoadThunk("B")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future2()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future3()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1", "A", "B"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not respect max batch size. Expected %#v, got %#v", expected, calls)
		}
	})

	t.Run("no cache does not cache anything", func(t *testing.T) {
		t.Skip("not supported yet")
		t.Parallel()
		identityLoader, loadCalls := NoCacheLoader(0)
		identityLoader.Prime("A", "Cached")
		identityLoader.Prime("B", "B")

		future1 := identityLoader.LoadThunk("1")
		future2 := identityLoader.LoadThunk("A")
		future3 := identityLoader.LoadThunk("B")

		_, err := future1()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future2()
		if err != nil {
			t.Error(err.Error())
		}
		_, err = future3()
		if err != nil {
			t.Error(err.Error())
		}

		calls := *loadCalls
		inner := []string{"1", "A", "B"}
		expected := [][]string{inner}
		if !reflect.DeepEqual(calls, expected) {
			t.Errorf("did not respect max batch size. Expected %#v, got %#v", expected, calls)
		}
	})
}

// test helpers
func IDLoader(max int) (*dataloadgen.Loader[string, string], *[][]string) {
	var mu sync.Mutex
	var loadCalls [][]string
	identityLoader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		mu.Lock()
		loadCalls = append(loadCalls, keys)
		mu.Unlock()
		for _, key := range keys {
			results = append(results, key)
		}
		return results, nil
	}, dataloadgen.WithBatchCapacity[string, string](max))
	return identityLoader, &loadCalls
}

func BatchOnlyLoader(max int) (*dataloadgen.Loader[string, string], *[][]string) {
	var mu sync.Mutex
	var loadCalls [][]string
	identityLoader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		mu.Lock()
		loadCalls = append(loadCalls, keys)
		mu.Unlock()
		for _, key := range keys {
			results = append(results, key)
		}
		return results, nil
	}, dataloadgen.WithBatchCapacity[string, string](max)) //dataloadgen.WithClearCacheOnBatch[string, string]())
	return identityLoader, &loadCalls
}
func ErrorLoader(max int) (*dataloadgen.Loader[string, string], *[][]string) {
	var mu sync.Mutex
	var loadCalls [][]string
	identityLoader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		mu.Lock()
		loadCalls = append(loadCalls, keys)
		mu.Unlock()
		for _, key := range keys {
			results = append(results, key)
			errs = append(errs, fmt.Errorf("this is a test error"))
		}
		return results, errs
	}, dataloadgen.WithBatchCapacity[string, string](max))
	return identityLoader, &loadCalls
}
func OneErrorLoader(max int) (*dataloadgen.Loader[string, string], *[][]string) {
	var mu sync.Mutex
	var loadCalls [][]string
	identityLoader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		results = make([]string, max)
		errs = make([]error, max)
		mu.Lock()
		loadCalls = append(loadCalls, keys)
		mu.Unlock()
		for i := range keys {
			var err error
			if i == 0 {
				err = errors.New("always error on the first key")
			}
			results[i] = keys[i]
			errs[i] = err
		}
		return results, errs
	}, dataloadgen.WithBatchCapacity[string, string](max))
	return identityLoader, &loadCalls
}
func PanicLoader(max int) (*dataloadgen.Loader[string, string], *[][]string) {
	var loadCalls [][]string
	panicLoader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		panic("Programming error")
	}, dataloadgen.WithBatchCapacity[string, string](max)) //, withSilentLogger())
	return panicLoader, &loadCalls
}
func BadLoader(max int) (*dataloadgen.Loader[string, string], *[][]string) {
	var mu sync.Mutex
	var loadCalls [][]string
	identityLoader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		mu.Lock()
		loadCalls = append(loadCalls, keys)
		mu.Unlock()
		results = append(results, keys[0])
		return results, nil
	}, dataloadgen.WithBatchCapacity[string, string](max))
	return identityLoader, &loadCalls
}

func NoCacheLoader(max int) (*dataloadgen.Loader[string, string], *[][]string) {
	var mu sync.Mutex
	var loadCalls [][]string
	//cache := &NoCache{}
	identityLoader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		mu.Lock()
		loadCalls = append(loadCalls, keys)
		mu.Unlock()
		for _, key := range keys {
			results = append(results, key)
		}
		return results, nil
	}, /*dataloadgen.WithCache[string, string](cache),*/ dataloadgen.WithBatchCapacity[string, string](max))
	return identityLoader, &loadCalls
}

// FaultyLoader gives len(keys)-1 results.
func FaultyLoader() (*dataloadgen.Loader[string, string], *[][]string) {
	var mu sync.Mutex
	var loadCalls [][]string

	loader := dataloadgen.NewLoader(func(keys []string) (results []string, errs []error) {
		mu.Lock()
		loadCalls = append(loadCalls, keys)
		mu.Unlock()

		lastKeyIndex := len(keys) - 1
		for i, key := range keys {
			if i == lastKeyIndex {
				break
			}

			results = append(results, key)
		}
		return results, nil
	})

	return loader, &loadCalls
}
