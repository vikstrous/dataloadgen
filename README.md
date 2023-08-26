# dataloadgen

[godoc](https://pkg.go.dev/github.com/vikstrous/dataloadgen)

`dataloadgen` is an implementation of a pattern popularized by [Facebook's Dataloader](https://github.com/graphql/dataloader).

It works as follows:
* A Loader object is created per graphql request.
* Each of many concurrently executing graphql resolver functions call Load() on the Loader object with different keys. Let's say K1, K2, K3
* Each call to Load() with a new key is delayed slightly (a few milliseconds) so that the Loader can load them together.
* The customizable fetch function of the loader takes a list of keys and loads data for all of them in a single batched request to the data storage layer. It might send `[K1,K2,K3]` and get back `[V1,V2,V3]`.
* The Loader takes case of sending the right result to the right caller and the result is cached for the duration of the graphql request.

Usage:

```sh
go get github.com/vikstrous/dataloadgen
```

See the usage [example](https://pkg.go.dev/github.com/vikstrous/dataloadgen#example-Loader) in the documentation:
```go
package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/vikstrous/dataloadgen"
)

// fetchFn is shown as a function here, but it might work better as a method
// ctx is the context from the first call to Load for the current batch
func fetchFn(ctx context.Context, keys []string) (ret []int, errs []error) {
    for _, key := range keys {
        num, err := strconv.ParseInt(key, 10, 32)
        ret = append(ret, int(num))
        errs = append(errs, err)
    }
    return
}

func main() {
    ctx := context.Background()
    // Per-request setup code:
    loader := dataloadgen.NewLoader(fetchFn)
    // In every graphql resolver:
    result, err := loader.Load(ctx, "1")
    if err != nil {
        panic(err)
    }
    fmt.Println(result)
}
```

## Comparison to others

[dataloaden](https://github.com/vektah/dataloaden) uses code generation and has similar performance
[dataloader](https://github.com/graph-gophers/dataloader) does not use code generation but has much worse performance and is more difficult to use

Benchmarks show that this package is faster than both of the above and I find it easier to use.

```
BenchmarkDataloader/caches-8                             4512438               267.7 ns/op           168 B/op          5 allocs/op
BenchmarkDataloader/random_spread-8                      1000000              1267 ns/op             626 B/op         11 allocs/op
BenchmarkDataloader/concurently-8                          16305             78332 ns/op           28619 B/op        154 allocs/op
BenchmarkDataloader/all_in_one_request-8                   10000           7081367 ns/op         2576182 B/op      60031 allocs/op

BenchmarkDataloaden/caches-8                            18537674                68.09 ns/op           24 B/op          1 allocs/op
BenchmarkDataloaden/random_spread-8                      2885696               696.8 ns/op           295 B/op          5 allocs/op
BenchmarkDataloaden/concurently-8                          23302             50965 ns/op            2799 B/op         75 allocs/op
BenchmarkDataloaden/all_in_one_request-8                   10000           1279412 ns/op          487867 B/op      10007 allocs/op

BenchmarkDataloadgen/caches-8                           23384140                53.00 ns/op            8 B/op          0 allocs/op
BenchmarkDataloadgen/random_spread-8                     2394097               604.9 ns/op           292 B/op          4 allocs/op
BenchmarkDataloadgen/concurently-8                         28359             39131 ns/op            2852 B/op         68 allocs/op
BenchmarkDataloadgen/all_in_one_request-8                  10000           1041825 ns/op          573643 B/op          8 allocs/op
```

To run the benchmarks, run `go test -bench=. . -benchmem` from the benchmark directory.