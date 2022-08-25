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
BenchmarkDataloader/caches-8                     4570850               250.1 ns/op           147 B/op          3 allocs/op
BenchmarkDataloader/random_spread-8               766516              1769 ns/op             744 B/op         17 allocs/op
BenchmarkDataloader/concurently-8                  13796             78000 ns/op           42645 B/op        229 allocs/op
BenchmarkDataloader/all_in_one_request-8            9380          11457205 ns/op         2279729 B/op      46967 allocs/op

BenchmarkDataloaden/caches-8                    21249274                63.10 ns/op           24 B/op          1 allocs/op
BenchmarkDataloaden/random_spread-8              1501452               879.7 ns/op           266 B/op          5 allocs/op
BenchmarkDataloaden/concurently-8                  22455             49690 ns/op            2910 B/op         75 allocs/op
BenchmarkDataloaden/all_in_one_request-8           10000           1228241 ns/op          487878 B/op      10007 allocs/op

BenchmarkDataloadgen/caches-8                   22181911                55.39 ns/op            8 B/op          0 allocs/op
BenchmarkDataloadgen/random_spread-8             2375029               457.1 ns/op           226 B/op          4 allocs/op
BenchmarkDataloadgen/concurently-8                 30919             36685 ns/op            2699 B/op         70 allocs/op
BenchmarkDataloadgen/all_in_one_request-8          10000           1042714 ns/op          573618 B/op          7 allocs/op
```