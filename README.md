# dataloadgen

This is a fork of https://github.com/vektah/dataloaden that uses generics
instead of code genation. See the docs there for the motivation and usage details.

With this package you can skip the generation step! That's why it's dataload`gen` (gen stands for generics).

To add this package as a dependency:

```
go get github.com/vikstrous/dataloadgen
```

See the example in the documentation: https://pkg.go.dev/github.com/vikstrous/dataloadgen

This package has evolved to include some ideas from dataloader https://github.com/graph-gophers/dataloader

Benchmarks show that this package is faster than both of the above and offers the best of both worlds.

```
BenchmarkDataloader/caches-8                     3963708               301.9 ns/op           162 B/op          4 allocs/op
BenchmarkDataloader/random_spread-8               642184              1961 ns/op             736 B/op         14 allocs/op
BenchmarkDataloader/concurently-8                  17191             85223 ns/op           45449 B/op        200 allocs/op

BenchmarkDataloaden/caches-8                    10976748               108.6 ns/op            26 B/op          1 allocs/op
BenchmarkDataloaden/random_spread-8              1000000              1107 ns/op             331 B/op          4 allocs/op
BenchmarkDataloaden/concurently-8                  23605             53127 ns/op            2984 B/op         68 allocs/op

BenchmarkDataloadgen/caches-8                   12434064                97.04 ns/op           10 B/op          0 allocs/op
BenchmarkDataloadgen/random_spread-8             1000000              1088 ns/op             307 B/op          3 allocs/op
BenchmarkDataloadgen/concurently-8                 39688             30788 ns/op            2729 B/op         60 allocs/op
```