# dataloadgen

Godoc: https://pkg.go.dev/github.com/vikstrous/dataloadgen

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
BenchmarkDataloader/caches-8                     4753599               252.5 ns/op           147 B/op          3 allocs/op
BenchmarkDataloader/random_spread-8               724077              1911 ns/op             758 B/op         17 allocs/op
BenchmarkDataloader/concurently-8                  71977             21800 ns/op           10477 B/op        179 allocs/op

BenchmarkDataloaden/caches-8                    17906731                65.20 ns/op           24 B/op          1 allocs/op
BenchmarkDataloaden/random_spread-8              1253845               852.8 ns/op           277 B/op          5 allocs/op
BenchmarkDataloaden/concurently-8                  25878             44226 ns/op            2807 B/op         75 allocs/op

BenchmarkDataloadgen/caches-8                   21556281                55.17 ns/op            8 B/op          0 allocs/op
BenchmarkDataloadgen/random_spread-8             2498688               479.0 ns/op           289 B/op          4 allocs/op
BenchmarkDataloadgen/concurently-8                 33300             34417 ns/op            2683 B/op         69 allocs/op
```