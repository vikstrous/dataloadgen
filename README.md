# dataloadgen

[godoc](https://pkg.go.dev/github.com/vikstrous/dataloadgen)

`dataloadgen` is an implementation of a pattern popularized by [Facebook's Dataloader](https://github.com/graphql/dataloader).

It works as follows:
* A Loader object is created per graphql request.
* Each of many concurrently executing graphql resolver functions call Load() on the Loader object with different keys. Let's say K1, K2, K3
* Each call to Load() with a new key is delayed slightly (a few milliseconds) so that the Loader can load them together.
* The customizable fetch function of the loader takes a list of keys and loads data for all of them in a single batched request to the data storage layer.
* The response is cached for the duration of the graphql request and returned to the caller.

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
func fetchFn(keys []string) (ret []int, errs []error) {
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
BenchmarkDataloader/caches-8                             4152324               270.3 ns/op           168 B/op          5 allocs/op
BenchmarkDataloader/random_spread-8                      1000000              1281 ns/op             626 B/op         11 allocs/op
BenchmarkDataloader/concurently-8                          33159             55575 ns/op           32649 B/op        160 allocs/op
BenchmarkDataloader/all_in_one_request-8                   10000           7556166 ns/op         2574411 B/op      60032 allocs/op

BenchmarkDataloaden/caches-8                            17960090                67.73 ns/op           24 B/op          1 allocs/op
BenchmarkDataloaden/random_spread-8                      1223949               955.0 ns/op           279 B/op          5 allocs/op
BenchmarkDataloaden/concurently-8                          27093             43594 ns/op            2867 B/op         76 allocs/op
BenchmarkDataloaden/all_in_one_request-8                   10000           1410499 ns/op          487876 B/op      10007 allocs/op

BenchmarkDataloadgen/caches-8                           22032517                53.61 ns/op            8 B/op          0 allocs/op
BenchmarkDataloadgen/random_spread-8                     2558128               483.7 ns/op           287 B/op          4 allocs/op
BenchmarkDataloadgen/concurently-8                         31900             34903 ns/op            2906 B/op         71 allocs/op
BenchmarkDataloadgen/all_in_one_request-8                  10000           1032841 ns/op          573619 B/op          7 allocs/op
```