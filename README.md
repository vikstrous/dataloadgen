# dataloadgen

This is a fork of https://github.com/vektah/dataloaden that uses generics
instead of code genation. See the docs there for the motivation and usage details.

With this package you can skip the generation step! That's why it's dataload`gen` (gen stands for generics).

To add this package as a dependency:

```
go get github.com/vikstrous/dataloadgen
```

See the example in the documentation: https://pkg.go.dev/github.com/vikstrous/dataloadgen

One enhancement over the origin is that this form uses functional options.

The benchmark BenchmarkLoaderFromDataloaden is copied from the upstream repo and the performance is unchanged:

```
pkg: github.com/vikstrous/dataloadgen
cpu: Intel(R) Xeon(R) CPU @ 3.10GHz
BenchmarkLoaderFromDataloaden/caches-8         	12597632	        94.85 ns/op	      10 B/op	       0 allocs/op
BenchmarkLoaderFromDataloaden/random_spread-8  	 1000000	      1047 ns/op	     315 B/op	       4 allocs/op
BenchmarkLoaderFromDataloaden/concurently-8    	   40258	     45855 ns/op	    5739 B/op	      68 allocs/op
```

The benchmark BenchmarkLoaderFromDataloader is copied from https://github.com/graph-gophers/dataloader and adapted.

Upstream performance:
```
BenchmarkLoader-8        1215384               968.0 ns/op
```

This repo's performance on the same test (modified to call LoadThunk instead of Load because they mean the same thing):
```
BenchmarkLoaderFromDataloader-8                   204468              6146 ns/op
```

The difference is explained by the lack of "input capacity" and excessive locking over the use of an input channel with capacity 1000.