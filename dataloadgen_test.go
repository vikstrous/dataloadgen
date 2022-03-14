package dataloadgen_test

import (
	"fmt"
	"strconv"
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
