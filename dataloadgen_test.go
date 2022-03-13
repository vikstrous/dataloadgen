package dataloadgen_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/vikstrous/dataloadgen"
)

func TestBasic(t *testing.T) {
	loader := dataloadgen.NewLoader(dataloadgen.LoaderConfig[string, int]{
		Wait:     1 * time.Millisecond,
		MaxBatch: 1,
		Fetch: func(keys []string) (ret []int, errs []error) {
			for _, key := range keys {
				num, err := strconv.ParseInt(key, 10, 32)
				ret = append(ret, int(num))
				errs = append(errs, err)
			}
			return
		},
	})
	one, err := loader.Load("1")
	if err != nil {
		t.Fatalf("failed to load: %s", err)
	}
	if one != 1 {
		t.Fatalf("wrong result: %d", one)
	}
}
