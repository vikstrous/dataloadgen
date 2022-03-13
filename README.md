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

The benchmark is copied from the upstream repo and the performance is unchanged:

```
pkg: github.com/vikstrous/dataloadgen
cpu: Intel(R) Xeon(R) CPU @ 3.10GHz
BenchmarkLoaderFromDataloaden/caches-8         	10416878	       116.0 ns/op	      34 B/op	       1 allocs/op
BenchmarkLoaderFromDataloaden/random_spread-8  	 1000000	      1062 ns/op	     348 B/op	       4 allocs/op
BenchmarkLoaderFromDataloaden/concurently-8    	   26421	     51821 ns/op	    3152 B/op	      68 allocs/op
```